Goal (incl. success criteria):

- Implement headless streaming for `compozy start` and `compozy fix-reviews` so they can run continuously in CI/sandboxes without Bubble Tea, while keeping TUI as the default in interactive terminals.
- Success requires CLI/config support, runtime validation updates, executor output-policy separation, regression coverage, and clean `make verify`.

Constraints/Assumptions:

- Follow `AGENTS.md` and `CLAUDE.md`; do not touch unrelated dirty worktree files.
- Accepted plan must be persisted under `.codex/plans/`.
- Skills in play: `brainstorming` already satisfied by the approved plan; implementation guarded by `golang-pro`, `bubbletea`, `refactoring-analysis`, `systematic-debugging`, `no-workarounds`, `testing-anti-patterns`, and final verification via `cy-final-verify`.
- `json` and `raw-json` will be enabled for workflow commands; `persist` and `run-id` remain `exec`-only.

Key decisions:

- Reuse the existing canonical runtime event bus/journal instead of building a second headless executor.
- Keep `--tui=true` by default for `start` and `fix-reviews`, but auto-disable it when no TTY is available unless the user explicitly forces `--tui=true`.
- Reuse the existing session block rendering/logging path for headless text streaming and add stdout JSONL streaming as a separate policy.

State:

- Completed after clean `make verify`.

Now:

- Prepare the final handoff with the verified behavior and test evidence.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/plans/2026-04-11-workflow-headless-streaming.md`
- `.codex/ledger/2026-04-11-MEMORY-workflow-headless.md`
- `internal/cli/{commands.go,commands_test.go,run.go,state.go,workspace_config.go,root_test.go,root_command_execution_test.go,testdata/start_help.golden,workspace_config_test.go}`
- `internal/core/workspace/{config_types.go,config_validate.go,config_test.go}`
- `internal/core/agent/{registry_validate.go,registry_test.go}`
- `internal/core/run/{executor/*,internal/runshared/config.go}`
- Commands: `rg`, `sed`, `gofmt`, `go test`, `make verify`

Done:

- Explored the current CLI, executor, UI, event-bus, journal, and `exec` headless implementation.
- Confirmed the root coupling: workflow execution treats human output and TUI as the same mode.
- Persisted the accepted implementation plan to `.codex/plans/2026-04-11-workflow-headless-streaming.md`.
- Added `--format text|json|raw-json` and `--tui` support to `start` and `fix-reviews`, including workspace config defaults and help text.
- Added workflow presentation normalization in the CLI: interactive terminals default to `tui=true`, non-TTY runs auto-disable TUI unless explicitly forced, and JSON formats reject TUI.
- Extended workspace and runtime validation so workflow modes accept `json`/`raw-json` while keeping `persist` and `run-id` restricted to `exec`.
- Split runtime presentation policy in `runshared.Config` so human text, Bubble Tea, and event streaming are independent decisions.
- Added workflow stdout event streaming in `internal/core/run/executor/event_stream.go`, with lean JSONL and raw canonical event modes backed by the existing bus/journal.
- Stopped workflow `json`/`raw-json` runs from printing a duplicate final result object on stdout; `result.json` artifacts remain persisted.
- Updated regression coverage:
  - help/config/validation tests in `internal/cli`, `internal/core/workspace`, and `internal/core/agent`
  - workflow CLI execution tests for `start --format json`, `fix-reviews --format raw-json`, and explicit `--tui` failures without TTY
  - executor tests for fallback event-bus creation and lean/raw workflow event streaming filters
- Verification passed:
  - `go test ./internal/cli ./internal/core/workspace ./internal/core/agent ./internal/core/run/executor -count=1`
  - `make verify`
