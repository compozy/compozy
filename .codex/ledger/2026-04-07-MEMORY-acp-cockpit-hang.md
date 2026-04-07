Goal (incl. success criteria):
- Fix the ACP cockpit hang shared by `compozy start` and `compozy fix-reviews` when the last task finishes but the TUI remains visually running and `q` / `Ctrl+C` no longer exits cleanly.
- Success requires a root-cause fix in the run lifecycle, regression coverage for normal completion vs shutdown teardown, and a clean `make verify`.

Constraints/Assumptions:
- Follow `AGENTS.md` and `CLAUDE.md`; do not touch unrelated dirty files.
- Required skills loaded for this bugfix: `golang-pro`, `systematic-debugging`, `no-workarounds`, `testing-anti-patterns`; `cy-final-verify` gates completion.
- Preserve current product behavior: after all jobs complete, the cockpit stays open until the user presses `q`.

Key decisions:
- Root cause is a normal-completion lifecycle race, not missing quit bindings: `awaitUIAfterCompletion()` closes the UI event adapter before the Bubble Tea model can consume the terminal `session.completed` / `job.completed` events.
- Keep normal completion and shutdown as separate paths: normal completion must leave event delivery alive until the user exits; drain/force shutdown still tears down the UI immediately.

State:
- Completed with clean `make verify`.

Done:
- Reproduced the failure from run artifacts: the latest stuck run reached `session.completed` and `job.completed` for the final task, but no terminal run artifact was written and the cockpit remained open.
- Verified existing unit/integration coverage already passes for drain/force shutdown and quit escalation.
- Confirmed the risky seam is `internal/core/run/executor/execution.go` `awaitUIAfterCompletion()`.

Now:
- Prepare the final handoff with verification evidence.

Next:
- None.

Open questions (UNCONFIRMED if needed):
- None.

Working set (files/ids/commands):
- `.codex/ledger/2026-04-07-MEMORY-acp-cockpit-hang.md`
- `internal/core/run/executor/execution.go`
- `internal/core/run/executor/execution_ui_test.go`
- `internal/core/run/executor/execution_acp_test.go`
- Verification commands: `go test ./internal/core/run/executor ./internal/core/run/ui ./internal/core/agent -count=1`, `make verify`

Done:
- Patched `awaitUIAfterCompletion()` so normal completion no longer closes UI events before the completed cockpit is rendered.
- Added a production comment documenting the normal-completion invariant and why early adapter shutdown is wrong.
- Updated executor/UI regression tests so normal completion keeps events open while drain/force shutdown still closes them.
- Ran targeted verification:
  - `go test ./internal/core/run/executor -count=1`
  - `go test ./internal/core/run/ui ./internal/core/run/executor ./internal/core/agent -count=1`
- Ran the full repository gate successfully: `make verify` passed with `0 issues`, `DONE 1004 tests`, and a successful build.
