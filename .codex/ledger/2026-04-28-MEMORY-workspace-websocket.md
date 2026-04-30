Goal (incl. success criteria):

- Replace the daemon-to-web workspace realtime channel from SSE/EventSource to WebSocket.
- Success means only workspace-level realtime invalidation moves to WebSocket; run detail streaming remains SSE; no SSE fallback remains for workspace events; targeted tests and `make verify` pass.

Constraints/Assumptions:

- User accepted the plan: workspace-only, direct replacement.
- No destructive git commands without explicit permission.
- Use `apply_patch` for manual edits.
- Add Go dependencies with `go get`, not manual `go.mod` edits.
- Accepted Plan Mode plan must be persisted under `.codex/plans/`.
- Final completion requires `cy-final-verify` and full `make verify`.
- Existing dirty worktree includes the prior `_meta.md` and workspace SSE changes; do not revert unrelated changes.

Key decisions:

- Keep `/api/runs/:run_id/stream` as SSE.
- Replace `/api/workspaces/:id/events` with `/api/workspaces/:id/ws`.
- Preserve the existing `WorkspaceEvent` payload semantics; change only the workspace browser transport envelope.
- Use `github.com/coder/websocket` for the Go WebSocket server.

State:

- Backend and frontend implementation complete; full verification passed.

Done:

- Read current ledger context and scanned existing ledgers.
- Read required implementation/testing skills.
- User chose workspace-only and direct replacement in Plan Mode.
- Persisted accepted Plan Mode plan.
- Added `github.com/coder/websocket` with `go get`.
- Replaced workspace route/contract from `/api/workspaces/:id/events` SSE to `/api/workspaces/:id/ws` WebSocket.
- Added JSON workspace socket envelopes for event, heartbeat, overflow, and error messages.
- Replaced the app-shell EventSource client with a WebSocket client that reconnects with bounded backoff and cancels on close/unmount.
- Enabled Vite proxy WebSocket forwarding.
- Regenerated web OpenAPI types.
- Targeted backend checks passed: `go test ./internal/api/core ./internal/api/contract ./internal/api/httpapi ./internal/daemon`.
- Targeted frontend checks passed: `bun run --cwd web typecheck` and app-shell workspace event tests.
- Lint initially found WebSocket cleanup/body-close issues and handler complexity; fixed by handling close errors, closing the dial response body, and splitting the workspace socket handler into focused helpers.
- Focused lint now passes: `golangci-lint run --allow-parallel-runners` reported `0 issues`.
- Full verification passed: `env -u NO_COLOR make verify` exited 0, including frontend lint/typecheck/test/build, Go fmt/lint/test/build, and Playwright e2e.

Now:

- Report final outcome and verification evidence.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-28-MEMORY-workspace-websocket.md`
- `.codex/plans/2026-04-28-workspace-websocket.md`
- `internal/api/core/*`
- `internal/api/contract/*`
- `internal/api/httpapi/*`
- `internal/daemon/workspace_events.go`
- `openapi/compozy-daemon.json`
- `web/src/systems/app-shell/*`
- `web/vite.config.ts`
