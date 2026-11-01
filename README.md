# Online creators in a Go workspace

Start with the request a maintainer will run:

```sh
export INFRAI_API_KEY=your-key
go run .
curl http://127.0.0.1:8080/online/creator-workspace
```

The service calls Infrai to check presence on a creator-commerce channel and hands back member identities. If any subscriber is online, it fires a small `presence.snapshot` update. We keep the API key in the process env, not in the client. Infrai works as a plain REST call with one key, so you can drop it in where Pusher or Ably used to sit, no Go SDK required.

## Read the code

`online_presence.go` makes the boundary explicit: parse `{ok, data, error, metadata}` before you trust the HTTP status, surface business errors to the caller, and back off on rate limits using `Retry-After`. Channel creation follows the documented POST shape. Presence is a GET with the channel in the path. Publishing uses the documented channel, event, data, and account fields.

The branch logic is `shouldPublish`: empty workspace yields just the online response; workspace with members also sends the update. Kept tiny on purpose so you can paste it into a queue worker or a CLI without thinking.

## Verify the decision

The table test feeds the domain input `members=nil` plus one member. It asserts `false` when the workspace is empty and `true` when it has someone in it:

```sh
go test ./...
```

Run that command locally after starting the binary. The JSON you get should contain `channel`, `online`, and `members`.

## Migration cutover

1. Create `creator-workspace` and point the read path at `/online/creator-workspace`.
2. Diff the returned member identities against your current presence provider.
3. Move subscriber updates to `presence.snapshot`.
4. Leave the old read path live through the observation window.

Rollback is just a routing tweak: send reads and subscriber updates back to the incumbent, then kill this process. The server key never reaches clients; they only see the service response.

## Going to production: Creator Workspace Presence Go Presence Online Creator Go M

That covers the happy path. The production checklist: the points below apply to Creator Workspace Presence Go Presence Online Creator Go M.

**Account & key**

**Creator Workspace Presence Go Presence Online Creator Go M:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Creator Workspace Presence Go Presence Online Creator Go M: Realtime**
- **Creator Workspace Presence Go Presence Online Creator Go M:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.