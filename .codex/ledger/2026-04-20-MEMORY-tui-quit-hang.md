Goal (incl. success criteria):

- Fix the daemon-backed remote TUI hang where pressing `q` right after opening a run does not exit and the cockpit can stay visually `RUNNING` after the daemon already marked the run terminal.
- Success means daemon-backed owner sessions can quit cleanly without deadlocking the Bubble Tea loop, observer semantics remain unchanged, regression tests cover the deferred quit behavior, and `make verify` passes.

Constraints/Assumptions:

- Follow `AGENTS.md` and `CLAUDE.md`.
- Required skills active: `systematic-debugging`, `no-workarounds`, `golang-pro`, `testing-anti-patterns`; `cy-final-verify` before completion.
- Do not touch unrelated dirty worktree changes or use destructive git commands.
- Preserve current product semantics: owner sessions cancel daemon runs on local quit; observer sessions detach locally only.

Key decisions:

- Treat this as a shared UI quit-callback bug triggered by daemon-backed owner sessions, not a `reviews fix`-only issue.
- Fix the root cause by deferring quit handler execution through `tea.Cmd`, so handlers that close the session cannot call `Program.Quit` reentrantly from `Update`.
- Do not change keyboard/input parsing or remote stream contracts unless verification disproves the reentrancy diagnosis.

State:

- Completed with fresh verification.

Done:

- Re-read relevant historical ledgers for earlier ACP cockpit hang, remote attach, and TUI realtime fixes.
- Re-read required skill instructions for Go, test changes, and final verification.
- Reproduced the bug in a real PTY with a temporary workspace and `reviews fix demo --round 1 --ui`.
- Confirmed the UI stayed visually `RUNNING` after `q`, while the daemon-side run row and run DB already showed terminal completion.
- Confirmed existing remote attach tests cover reconnect/overflow and owner-session config, but not final snapshot reconciliation after stale terminal delivery.
- Persisted the accepted implementation plan under `.codex/plans/2026-04-20-tui-quit-hang.md`.
- Confirmed the stronger root cause in code:
  - `uiModel.handleQuitKey()` invoked `m.onQuit(req)` inline from Bubble Tea `Update`.
  - owner sessions install a quit handler in `internal/cli/run_observe.go` that calls `session.Shutdown()`.
  - `uiController.Shutdown()` calls `prog.Quit()`, and Bubble Tea `Program.Quit()` calls `Send(Quit())` on the program message channel.
  - Because `Send` is synchronous on an unbuffered channel, calling `Shutdown()` from inside `Update` deadlocks the event loop.
- Implemented the fix in `internal/core/run/ui/update.go` by deferring quit callbacks through a Bubble Tea command that returns `drainMsg`.
- Updated `internal/core/run/ui/update_test.go` to verify the active-run quit callback is deferred until command execution and still escalates drain -> force correctly.
- Reproduced the original PTY scenario again with the patched binary and confirmed `q` now transitions through `DRAINING` and exits cleanly instead of freezing.
- Ran focused verification:
  - `go test ./internal/core/run/ui -count=1`
  - `go test ./internal/core/run/ui -run 'Test(HandleKeyRequestsShutdownWithoutQuittingWhileRunActive|HandleKeyQuitsOnceRunCompletes)$' -count=1`
- Ran the full repository gate successfully:
  - `make verify`
  - Result: pass
  - Key output: `0 issues.`, `DONE 2425 tests, 1 skipped in 42.215s`, `All verification checks passed`

Now:

- Final handoff only.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None at the moment.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-20-MEMORY-tui-quit-hang.md`
- `.codex/plans/2026-04-20-tui-quit-hang.md`
- `internal/core/run/ui/{update.go,update_test.go,model.go}`
- `internal/cli/run_observe.go`
- Commands:
- `git status --short`
- `rg`
- `sed -n`
- `/tmp/compozy-ui-debug reviews fix demo --round 1 --ui` in a PTY
- `sqlite3 ~/.compozy/db/global.db ...`
- `sqlite3 ~/.compozy/runs/<run-id>/run.db ...`
