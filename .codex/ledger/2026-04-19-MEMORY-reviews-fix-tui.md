Goal (incl. success criteria):

- Keep daemon-backed run UX correct end-to-end.
- Success means:
  - remote TUI attach no longer opens blank/stale cockpits for active or already-settled runs;
  - `runs attach` and started daemon-backed runs fall back to textual replay when the snapshot is already settled;
  - `compozy daemon stop` cancels active runs by default instead of surfacing a conflict in the normal CLI path;
  - closing a TUI that started a daemon-backed run cancels that run instead of leaving it running in background;
  - fresh `make verify` passes.

Constraints/Assumptions:

- Follow `AGENTS.md` and `CLAUDE.md`.
- Required skills this session: `systematic-debugging`, `no-workarounds`, `golang-pro`, `testing-anti-patterns`; `cy-final-verify` before claiming completion.
- Do not use destructive git commands or touch unrelated worktree changes.
- Treat user screenshots/reports as symptoms only; confirm behavior in code/tests before fixing.

Key decisions:

- Treat the original blank cockpit as two separate attach races:
  - settled-before-attach: handled by CLI fallback from UI attach to textual replay;
  - live-run-with-empty-snapshot: handled in the TUI model by materializing placeholder jobs from later indexed events.
- Keep `runs attach` as an observer flow; it must not cancel daemon runs on local exit.
- Treat TUI sessions opened by commands that started daemon runs as owner sessions; local exit must request daemon-side cancel.
- Fix `daemon stop` in the CLI contract by defaulting the operator-facing command to cancel active runs before stopping.

State:

- Completed.

Done:

- Fixed settled-snapshot remote attach fallback in `internal/cli/{run_observe.go,daemon_commands.go,runs.go}`.
- Fixed remote UI placeholder hydration for indexed events without prior `job.queued` in `internal/core/run/ui/update.go`.
- Added regression coverage for the attach races in:
  - `internal/cli/daemon_commands_test.go`
  - `internal/cli/root_command_execution_test.go`
  - `internal/core/run/ui/update_test.go`
  - `internal/core/run/ui/remote_test.go`
- Added daemon client run cancellation support in `internal/api/client/runs.go`.
- Made started daemon-backed TUI sessions owner-aware in `internal/cli/run_observe.go`:
  - observer attach still uses `attachCLIRunUI`;
  - owner attach uses `attachStartedCLIRunUI`;
  - local TUI exit now requests daemon-side run cancellation before returning.
- Extended remote attach options with `OwnerSession` in `internal/core/run/ui/remote.go` so owner sessions preserve local quit handling.
- Changed `compozy daemon stop` to default `--force=true` at the CLI layer in `internal/cli/daemon.go`.
- Added focused regressions for the new lifecycle semantics:
  - `TestDefaultAttachStartedCLIRunUICancelsOwnedRunOnLocalExit`
  - `TestDaemonStopCommandCancelsActiveRunsByDefault`
  - `TestAttachRemoteKeepsOwnerSessionsCancelableFromLocalQuit`
- Ran focused validation:
  - `go test ./internal/cli ./internal/core/run/ui -run 'Test(DefaultAttachStartedCLIRunUICancelsOwnedRunOnLocalExit|DaemonStopCommandCancelsActiveRunsByDefault|HandleStartedTaskRunFallsBackToWatchWhenUIAttachIsAlreadySettled|RunsAttachCommandFallsBackToWatchWhenRunIsAlreadySettled|AttachRemoteKeepsOwnerSessionsCancelableFromLocalQuit|AttachRemoteSkipsLiveStreamForCompletedSnapshot)$' -count=1`
  - result: pass
- Ran full gate:
  - `make verify`
  - result: pass
  - key output: `0 issues.`, `DONE 2410 tests, 1 skipped in 41.079s`, `All verification checks passed`

Now:

- No technical work remains; only final handoff.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: `exec --tui` still follows a different client path (`waitAndPrintExecResult`) and may deserve a separate UX audit if the expected behavior is true interactive attach/cancel semantics there as well.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-19-MEMORY-reviews-fix-tui.md`
- `internal/api/client/runs.go`
- `internal/cli/{daemon.go,daemon_commands.go,daemon_commands_test.go,daemon_exec_test_helpers_test.go,run_observe.go}`
- `internal/core/run/ui/{aliases.go,remote.go,remote_test.go}`
- Commands:
  - `go test ./internal/cli ./internal/core/run/ui -run 'Test(DefaultAttachStartedCLIRunUICancelsOwnedRunOnLocalExit|DaemonStopCommandCancelsActiveRunsByDefault|HandleStartedTaskRunFallsBackToWatchWhenUIAttachIsAlreadySettled|RunsAttachCommandFallsBackToWatchWhenRunIsAlreadySettled|AttachRemoteKeepsOwnerSessionsCancelableFromLocalQuit|AttachRemoteSkipsLiveStreamForCompletedSnapshot)$' -count=1`
  - `make verify`
