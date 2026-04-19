Goal (incl. success criteria):

- Change the daemon HTTP default port from `0` (ephemeral) to `2323`.
- Success means: daemon startup without an explicit port binds to `2323`, daemon info/status reflect that default, regression coverage exists, and `make verify` passes.

Constraints/Assumptions:

- Follow repository instructions from `AGENTS.md` and `CLAUDE.md`.
- Required skills for this session: `systematic-debugging`, `no-workarounds`, `golang-pro`; `testing-anti-patterns` is also required because tests will be modified.
- Do not touch unrelated worktree changes under `.agents/skills/smux-compozy-pairing/*` and unrelated ledger files.
- No destructive git commands. Completion requires fresh `make verify`.

Key decisions:

- Root cause is not in daemon info persistence; it is in daemon transport composition.
- `internal/daemon/startHostTransports` creates the HTTP server without `httpapi.WithPort(...)`, so `httpapi.New()` falls back to its internal `port: 0` default and the listener binds an ephemeral port.
- The fix should make the daemon’s default explicit at the daemon/domain boundary rather than relying on an implicit transport default.
- The environment already has an unrelated external `compozy daemon start` listening on `127.0.0.1:2323`, so real CLI integration tests cannot assume the global default port is free.
- To keep the product default at `2323` without mutating the user environment, the CLI now honors `COMPOZY_DAEMON_HTTP_PORT` as an explicit override for isolated test/runtime contexts.

State:

- In progress.

Done:

- Read relevant cross-agent daemon ledgers for context.
- Inspected worktree status to avoid unrelated changes.
- Traced daemon startup from CLI to host runtime and confirmed where port `0` originates.
- Identified test surfaces in `internal/api/httpapi/transport_integration_test.go` and daemon startup surfaces in `internal/daemon/{boot.go,host.go}`.
- Added regression coverage for the desired default:
  - `internal/daemon/boot_test.go::TestStartDefaultsHTTPPortWhenUnset`
  - `internal/cli/operator_commands_integration_test.go::TestDaemonStatusAndStopCommandsOperateAgainstRealDaemon`
- Reproduced the bug with focused tests:
  - `go test ./internal/daemon -run TestStartDefaultsHTTPPortWhenUnset -count=1` failed with `Info.HTTPPort = 0, want 2323`
  - `go test ./internal/cli -run TestDaemonStatusAndStopCommandsOperateAgainstRealDaemon -count=1` failed with daemon status `http_port = 55646`
- Ran `make verify`; lint initially failed on an unrelated existing `goconst` issue in `internal/cli/reviews_exec_daemon.go`, fixed by introducing `execStatusSucceeded`.
- Reproduced and diagnosed the full-suite CLI failure after the port change:
  - `go test ./internal/cli -run 'Test(DaemonStatusAndStopCommandsOperateAgainstRealDaemon|WorkspaceCommandsReflectDaemonRegistryAgainstRealDaemon|WorkspacesUnregisterRejectsActiveRunsAgainstRealDaemon|SyncAndArchiveCommandsUseDaemonStateFromWorkspaceSubdirectory)' -count=1` failed because daemon auto-start could not create `daemon.json`
  - `lsof -nP -iTCP:2323 -sTCP:LISTEN` showed an external `compozy` process already bound to `127.0.0.1:2323`
- Added CLI override plumbing via `COMPOZY_DAEMON_HTTP_PORT` and updated real CLI integration tests to reserve isolated ports before auto-start.
- Re-ran focused validation successfully:
  - `go test ./internal/daemon -run TestStartDefaultsHTTPPortWhenUnset -count=1`
  - `go test ./internal/cli -run 'Test(DaemonStatusAndStopCommandsOperateAgainstRealDaemon|WorkspaceCommandsReflectDaemonRegistryAgainstRealDaemon|WorkspacesUnregisterRejectsActiveRunsAgainstRealDaemon|SyncAndArchiveCommandsUseDaemonStateFromWorkspaceSubdirectory|ArchiveCommandArchivesSyncedWorkflowIntoNewPathFormat)' -count=1`

Now:

- Run `make verify` again after the CLI port-override follow-up.

Next:

- Run focused daemon/http transport tests, then run `make verify`.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: whether documentation should also mention `2323`; code search has not yet found a user-facing port-default statement.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-18-MEMORY-daemon-default-port.md`
- `.codex/ledger/2026-04-18-MEMORY-daemon-command-surface.md`
- `.codex/ledger/2026-04-17-MEMORY-daemon-architecture.md`
- `internal/daemon/host.go`
- `internal/daemon/boot.go`
- `internal/api/httpapi/server.go`
- `internal/api/httpapi/transport_integration_test.go`
- `internal/daemon/boot_test.go`
- `git status --short`
- `rg -n "HTTPPort|WithPort|daemon start|http_port"`
