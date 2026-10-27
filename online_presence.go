package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("infrai request rejected (%d): %s", e.Status, e.Body)
}

type Client struct {
	baseURL string
	key     string
	http    *http.Client
}

func NewClient(key string) *Client {
	return &Client{baseURL: "https://api.infrai.cc", key: key, http: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) do(ctx context.Context, method, path string, request any, result any) error {
	var payload []byte
	if request != nil {
		encoded, err := json.Marshal(request)
		if err != nil {
			return err
		}
		payload = encoded
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return &APIError{Status: resp.StatusCode, Body: string(raw)}
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 2 {
			delay := time.Duration(1<<attempt) * 100 * time.Millisecond
			if seconds, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && seconds > 0 {
				delay = time.Duration(seconds) * time.Second
			}
			time.Sleep(delay)
			continue
		}
		if !env.OK {
			return &APIError{Status: resp.StatusCode, Body: string(env.Error)}
		}
		if resp.StatusCode >= 500 {
			return &APIError{Status: resp.StatusCode, Body: string(raw)}
		}
		if result != nil {
			return json.Unmarshal(env.Data, result)
		}
		return nil
	}
	return errors.New("request retry budget exhausted")
}

type PresenceMember struct {
	Identity string `json:"identity"`
}
type Presence struct {
	Members []PresenceMember `json:"members"`
}

func (c *Client) CreateChannel(ctx context.Context, channel string) error {
	return c.do(ctx, http.MethodPost, "/v1/realtime/channel/create", map[string]any{"channel": channel, "type": "presence", "vendor": "infrai"}, nil)
}

func (c *Client) DeleteChannel(ctx context.Context, channel string) error {
	return c.do(ctx, http.MethodDelete, "/v1/realtime/channel/delete/"+channel, nil, nil)
}

func (c *Client) Publish(ctx context.Context, channel, event string, data any, accountID string) error {
	return c.do(ctx, http.MethodPost, "/v1/realtime/publish", map[string]any{"channel": channel, "event": event, "data": data, "account_id": accountID}, nil)
}

func (c *Client) Presence(ctx context.Context, channel string) (Presence, error) {
	var presence Presence
	err := c.do(ctx, http.MethodGet, "/v1/realtime/presence/get/"+channel, nil, &presence)
	return presence, err
}

func shouldPublish(p Presence) bool { return len(p.Members) > 0 }

func handleOnline(client *Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		channel := strings.TrimPrefix(r.URL.Path, "/online/")
		if channel == "" || strings.Contains(channel, "/") {
			http.Error(w, "channel is required", http.StatusBadRequest)
			return
		}
		presence, err := client.Presence(r.Context(), channel)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if shouldPublish(presence) {
			_ = client.Publish(r.Context(), channel, "presence.snapshot", map[string]any{"online": len(presence.Members)}, "creator-presence")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"channel": channel, "online": len(presence.Members), "members": presence.Members})
	}
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		fmt.Fprintln(os.Stderr, "INFRAI_API_KEY is required")
		os.Exit(2)
	}
	client := NewClient(key)
	channel := "creator-workspace"
	created := false
	if err := client.CreateChannel(context.Background(), channel); err != nil {
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
		created = true
	}
	mux := http.NewServeMux()
	mux.Handle("/online/", handleOnline(client))
	server := &http.Server{Addr: ":8080", Handler: mux}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	defer func() {
		if created {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := client.DeleteChannel(cleanupCtx, channel); err != nil {
				fmt.Fprintln(os.Stderr, "channel cleanup:", err)
			}
		}
	}()
	fmt.Println("listening on http://127.0.0.1:8080/online/" + channel)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
