# Task Memory: task_02.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Implemented task 02 end to end: runtime precedence for reusable agents, canonical system prompt assembly, compact discovery catalog generation, shared-path integration for workflow and exec execution, focused tests, and clean verification.

## Important Decisions
- Added `model.ExplicitRuntimeFlags` and threaded it through `core.Config` so reusable-agent defaults can honor Cobra-style explicitness instead of raw field values.
- Added `internal/core/agents.ResolveExecutionContext` as the reusable task-02 seam. It resolves the selected agent, applies runtime precedence, and produces the canonical prompt by extending an existing base system prompt instead of replacing it.
- Reused the same agent execution context in both `internal/core/plan/prepare.go` and `internal/core/run/exec/exec.go` because exec mode does not go through planning.

## Learnings
- Existing workspace/default runtime values can remain in `RuntimeConfig`; the only extra signal required for correct precedence is whether a runtime field was explicitly set.
- PRD-task workflow memory fits naturally as the built-in framing section of the canonical agent system prompt when the selected agent is injected during batch preparation.
- The shared ACP path exposes runtime/access resolution in two places: `agent.ClientConfig` carries IDE/reasoning/access mode, while `agent.SessionRequest` carries the final composed prompt and session-level model override.

## Files / Surfaces
- `internal/core/model/runtime_config.go`
- `internal/core/api.go`
- `internal/cli/{state.go,run.go,root_test.go}`
- `internal/core/agents/{execution.go,execution_test.go}`
- `internal/core/plan/{prepare.go,prepare_test.go}`
- `internal/core/run/exec/{exec.go,exec_integration_test.go}`
- `internal/core/kernel/commands/commands_test.go`

## Errors / Corrections
- `make verify` initially failed on new `gocritic` findings in `internal/core/agents/execution.go` (`appendCombine` and `rangeValCopy`). Fixed by combining adjacent `append` calls and iterating over catalog entries by index/pointer.

## Ready for Next Run
- Task 02 is complete. Task 03 and task 04 should build on `ResolveExecutionContext`, `RuntimeConfig.AgentName`, and `ExplicitRuntimeFlags` rather than reintroducing agent-specific precedence logic in separate execution paths.
