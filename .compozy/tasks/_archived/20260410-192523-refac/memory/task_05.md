# Task Memory: task_05.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Land the Phase 4 DRY/generics consolidation without behavior changes: shared content-block engine, generic CLI/kernel/setup helpers, collapsed runtime-config chain, declarative setup tables, and parameter objects for ACP session setup/update handling.
- Completed with a clean `make verify`; only the local source-code commit remains after tracking updates.

## Important Decisions
- Put shared block encode/decode/validation logic in `internal/contentblock` so both `internal/core/model` and public `pkg/compozy/events/kinds` can import it without `internal` visibility issues.
- Collapse the config chain by moving kernel commands to `model.RuntimeConfig` and keeping `core.Config.RuntimeConfig()` as the single conversion entry point.
- Rebuild `internal/setup/agents.go` as declarative path specs plus generic `selectByName[T]`, while preserving existing alias behavior for `claude` -> `claude-code`.
- Replace ACP high-arity helper signatures with `SessionSetupRequest` and `SessionUpdateHandlerConfig` in `internal/core/run/internal/acpshared`; keep root/run alias layers thin.

## Learnings
- A shared root-level internal package was required for content blocks because `pkg/compozy/events/kinds` cannot import anything under `internal/core/...`.
- Embedding sub-structs in `internal/cli/commandState` preserves most existing selector code through field promotion, but composite literals in tests must initialize the embedded structs explicitly.
- The setup refactor is safest when it preserves exact user-facing error phrasing (`invalid skill(s)`, `invalid agent(s)`), even though the selection engine is now generic.

## Files / Surfaces
- `internal/contentblock/engine.go`
- `internal/core/model/content.go`
- `pkg/compozy/events/kinds/content_block.go`
- `internal/cli/{commands_simple.go,form.go,state.go,workspace_config.go,workspace_config_test.go,root_test.go}`
- `internal/core/api.go`
- `internal/core/kernel/{core_adapters.go,handlers.go}`
- `internal/core/kernel/commands/{run_start.go,workflow_prepare.go,runtime_config.go,commands_test.go}`
- `internal/setup/{agents.go,agents_test.go,install.go,select.go}`
- `internal/core/run/exec/{aliases.go,exec.go}`
- `internal/core/run/internal/acpshared/{command_io.go,command_io_test.go,session_exec.go,session_handler.go,session_handler_test.go}`

## Errors / Corrections
- `internal/setup/agents.go` was temporarily deleted while converting the closure slab to declarative data; the correction was to fully recreate it before continuing and add direct setup tests for representative agents.
- The first runtime test pass after the ACP parameter-object refactor exposed stale imports in `exec/aliases.go` and `compat_test.go`; trimming those imports restored a green `go test ./internal/core/run/...`.

## Ready for Next Run
- `make verify` passed cleanly after the final lint/test fixes.
- Local commit `a62de93` (`refactor: consolidate generics and shared runtime plumbing`) contains the source/test changes for this task.
- Workflow tracking artifacts remain intentionally unstaged so the automatic commit contains only production/test code.
