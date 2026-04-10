Goal (incl. success criteria):

- Complete `task_05` Phase 4 DRY/generics consolidation: unify content-block machinery, replace repeated CLI/kernel/setup helpers with generics or shared abstractions, collapse the config translation chain, introduce parameter objects for high-arity run helpers, preserve behavior, and finish with clean `make verify`.

Constraints/Assumptions:

- Follow `AGENTS.md`, `CLAUDE.md`, `.compozy/tasks/refac/task_05.md`, `.compozy/tasks/refac/_techspec.md`, `.compozy/tasks/refac/_tasks.md`, and workflow memory under `.compozy/tasks/refac/memory/`.
- Required skills loaded: `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `no-workarounds`, `systematic-debugging`, `testing-anti-patterns`; `cy-final-verify` must gate completion.
- Workspace is already dirty from earlier task tracking files; do not touch unrelated changes.
- No ADR directory exists under `.compozy/tasks/refac/adrs` at start of run.
- Task scope is structural refactoring only; do not change external behavior.

Key decisions:

- Use a shared `internal/contentblock` package to preserve both camelCase (`internal/core/model`) and snake_case (`pkg/compozy/events/kinds`) JSON surfaces while centralizing decode/encode/normalize logic.
- Collapse the kernel config translation chain by moving commands from `commands.RuntimeFields` to `model.RuntimeConfig`, with `core.Config.RuntimeConfig()` as the remaining conversion entry point.
- Preserve Task 04 runtime ownership boundaries (`internal/core/run/{exec,executor,transcript,ui}` and `internal/core/run/internal/*`) while introducing the required ACP parameter objects inside `internal/core/run/internal/acpshared`.
- Rebuild `internal/setup/agents.go` as declarative path data plus a shared generic `selectByName` helper instead of closure-heavy agent specs.

State:

- In progress with all production refactors landed and targeted package suites green; repository-wide verification, tracking updates, and local commit remain.

Done:

- Read repo guidance, workflow memory, `task_05.md`, `_techspec.md`, `_tasks.md`, and the cited report findings (`20260406-provider-public.md` F01, `20260406-core-foundation.md` F2/F4/F5/F7, `20260406-cli-entry.md` F2/F3/F4).
- Scanned the existing cross-agent ledger from `task_04` for runtime package boundary context.
- Verified the workspace is dirty before changes; task tracking files from earlier phases are already modified.
- Added `internal/contentblock` and moved content-block encode/decode/validation logic there; both `internal/core/model/content.go` and `pkg/compozy/events/kinds/content_block.go` now delegate to it with compatibility tests for camelCase and snake_case JSON.
- Replaced duplicated CLI helpers with generics: `applyConfig[T]`, `applyInput[T]`, extracted `simpleCommandBase`, and decomposed `commandState` into embedded sub-structs.
- Removed `commands.RuntimeFields`, added `commands/runtime_config.go`, converted kernel commands/adapters/handlers to generic/shared forms, and switched command runtime payloads to `model.RuntimeConfig`.
- Restored `internal/setup/agents.go` as a declarative table, added generic `selectByName`, and covered alias/dedup/path behavior with setup tests.
- Replaced the ACP 13-parameter setup/handler helpers with `SessionSetupRequest` and `SessionUpdateHandlerConfig`, updated alias layers/callers, and added direct request-object coverage in `command_io_test.go`.
- Targeted verification passed for:
  - `go test ./internal/contentblock ./internal/core/model ./pkg/compozy/events/kinds ./internal/core/contentconv`
  - `go test ./internal/cli`
  - `go test ./internal/core/kernel/...`
  - `go test ./internal/setup`
  - `go test ./internal/core/run/...`
- Earlier in this session, `make verify` also passed after the content-block consolidation.

Now:

- Update workflow/task memory with the landed decisions and touched surfaces, then run the repository-wide verification gate.

Next:

- If `make verify` passes, update task tracking (`task_05.md`, `_tasks.md`) and create the required local commit.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.compozy/tasks/refac/task_05.md`
- `.compozy/tasks/refac/_techspec.md`
- `.compozy/tasks/refac/_tasks.md`
- `.compozy/tasks/refac/20260406-provider-public.md`
- `.compozy/tasks/refac/20260406-core-foundation.md`
- `.compozy/tasks/refac/20260406-cli-entry.md`
- `.compozy/tasks/refac/memory/MEMORY.md`
- `.compozy/tasks/refac/memory/task_05.md`
- `.codex/ledger/2026-04-06-MEMORY-dry-generics.md`
- `internal/contentblock/*`
- `internal/cli/{commands_simple.go,form.go,state.go,workspace_config.go,workspace_config_test.go,root_test.go}`
- `internal/core/kernel/{core_adapters.go,handlers.go}`
- `internal/core/kernel/commands/{run_start.go,workflow_prepare.go,runtime_config.go,commands_test.go}`
- `internal/core/api.go`
- `internal/setup/{agents.go,agents_test.go,install.go,select.go}`
- `internal/core/run/internal/acpshared/{command_io.go,command_io_test.go,session_exec.go,session_handler.go,session_handler_test.go}`
- `internal/core/run/exec/{aliases.go,exec.go}`
- Verification commands: `go test ./internal/cli`, `go test ./internal/core/kernel/...`, `go test ./internal/setup`, `go test ./internal/core/run/...`
