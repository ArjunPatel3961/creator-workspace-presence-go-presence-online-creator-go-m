package main

import "testing"

func TestShouldPublish(t *testing.T) {
	tests := []struct {
		name    string
		members []PresenceMember
		want    bool
	}{
		{name: "workspace has subscribers", members: []PresenceMember{{Identity: "alice"}}, want: true},
		{name: "workspace is empty", members: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldPublish(Presence{Members: tt.members})
			if got != tt.want {
				t.Fatalf("shouldPublish() = %v, want %v", got, tt.want)
			}
		})
	}
}
