Goal (incl. success criteria):

- Implement a Bubble Tea quit-confirmation modal so active runs no longer stop automatically when the operator presses `q` / `Ctrl+C`.
- Success means active-run quit opens a modal with default action "close TUI and keep run running", explicit stop still shuts the run down, observer semantics stay unchanged, regression tests cover the new flow, and `make verify` passes.

Constraints/Assumptions:

- Follow `AGENTS.md`, `CLAUDE.md`, and the accepted plan persisted under `.codex/plans/2026-04-20-tui-quit-modal.md`.
- Required skills active for this task: `bubbletea`, `systematic-debugging`, `no-workarounds`, `refactoring-analysis`, `golang-pro`, `testing-anti-patterns`, `cy-final-verify`.
- Do not touch unrelated dirty files or use destructive git commands.
- Existing deferred quit callback fix in `internal/core/run/ui/update.go` is part of the current worktree and must be preserved.

Key decisions:

- Apply the modal to every active run shown in the TUI, not only daemon-backed owner sessions.
- Keep `QuitRequest{Drain,Force}` meaning "explicit stop only"; closing the TUI without stopping the run must not emit a quit request.
- Use a small UI state machine in `internal/core/run/ui` instead of pushing more branching into CLI / executor code.
- Reuse the existing ACP panel styling and centered-dialog pattern used elsewhere in the repo.

State:

- Completed with fresh verification.

Done:

- Re-read the existing TUI quit-hang ledger for prior context.
- Re-read required implementation / testing / verification skills.
- Confirmed `tea.Program.Send` is safe after program exit (`bubbletea/v2` no-op when the program has terminated), so local detach via `tea.Quit` is feasible.
- Confirmed the active dirty worktree changes are limited to the prior deferred-quit fix in `internal/core/run/ui/update.go` and `update_test.go`.
- Persisted the accepted quit-modal plan under `.codex/plans/2026-04-20-tui-quit-modal.md`.
- Added explicit quit-dialog state to the ACP TUI model and wired active-run `q` / `Ctrl+C` to open the dialog instead of immediately stopping the run.
- Kept `DetachOnly` and completed-run quit behavior unchanged.
- Reused the existing deferred stop callback path for explicit "Stop Run" confirmation, preserving drain -> force escalation semantics.
- Added centered quit-dialog rendering plus footer label updates (`EXIT` before shutdown, `FORCE QUIT` once draining).
- Added UI regression tests for:
  - opening the dialog on active-run quit
  - default close-without-stop behavior
  - explicit stop behavior and force escalation
  - dialog dismissal on completion and external shutdown
  - detach-only direct quit behavior
- Added view regression tests for active-run footer text and quit-dialog copy.
- Added CLI regression coverage proving owner sessions no longer call `CancelRun` when the TUI closes without an explicit stop request.
- Ran focused verification successfully:
  - `go test ./internal/core/run/ui -count=1`
  - `go test ./internal/cli -run 'TestDefaultAttachStartedCLIRunUI(CancelsOwnedRunOnLocalExit|DoesNotCancelOwnedRunWhenUICloseDoesNotRequestStop)$' -count=1`
- Ran the full repository gate successfully:
  - `make verify`
  - Result: pass
  - Key output: `0 issues.`, `DONE 2431 tests, 1 skipped in 42.164s`, `All verification checks passed`

Now:

- Final handoff only.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/plans/2026-04-20-tui-quit-modal.md`
- `.codex/ledger/2026-04-20-MEMORY-tui-quit-modal.md`
- `internal/core/run/ui/{types.go,update.go,view.go,view_test.go,update_test.go}`
- `internal/cli/daemon_commands_test.go`
- Commands:
- `git status --short`
- `git diff -- ...`
- `sed -n`
- `rg`
