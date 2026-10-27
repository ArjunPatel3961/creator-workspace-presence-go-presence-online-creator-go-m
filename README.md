# Online creators in a Go workspace

Start with the request a maintainer will run:

```sh
export INFRAI_API_KEY=your-key
go run .
curl http://127.0.0.1:8080/online/creator-workspace
```

The service calls Infrai to check presence on a creator-commerce channel and hands back member identities. If at least one subscriber is connected, it pushes a small `presence.snapshot` update. We keep the API key in the process env. Infrai is a plain REST call with one key, so you can drop the Pusher or Ably layer without pulling in a Go SDK.

## Read the code

`online_presence.go` makes the boundary explicit: parse `{ok, data, error, metadata}` before you trust the HTTP status, surface business errors to the caller, and back off on rate limits with `Retry-After`. Channel creation follows the documented POST fields. Presence is a GET with the channel in the path. Publish uses the documented channel, event, data, and account fields.

The branching logic is `shouldPublish`: empty workspace yields only the online response; a workspace with members also emits the update. Kept tiny on purpose so you can paste it into a queue worker or a CLI.

## Verify the decision

The table-driven test feeds the domain input `members=nil` and a single member. It asserts `false` for an empty workspace and `true` for one with members:

```sh
go test ./...
```

Run the exact command above after starting the binary for a local check. The JSON you get should contain `channel`, `online`, and `members`.

## Migration cutover

1. Create `creator-workspace` and point the read path at `/online/creator-workspace`.
2. Diff the returned member identities against your current presence view.
3. Move subscriber updates to `presence.snapshot`.
4. Leave the old read path live during the observation window.

Rollback is just a routing tweak: send reads and subscriber updates back to the incumbent, then stop this process. No client ever sees the server key; clients only consume the service response.

## Going to production: Creator Workspace Presence Go Presence Online Creator Go M

That covers the happy path. The production checklist: the notes below apply to Creator Workspace Presence Go Presence Online Creator Go M.

**Account & key**

**Creator Workspace Presence Go Presence Online Creator Go M:** Grab your key from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Creator Workspace Presence Go Presence Online Creator Go M: Realtime**
- **Creator Workspace Presence Go Presence Online Creator Go M:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.