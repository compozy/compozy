# Task Memory: task_05.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Harden reusable-agent execution with structured observability, stable blocked-reason reporting, and end-to-end tests for success, resume, cycle/depth blocking, invalid MCP config, and workspace override flows.
- Completion gate remains task 05 requirements plus clean `make verify`.
- Status: implementation, targeted coverage, and full verification are complete.

## Important Decisions
- Keep implementation aligned with existing session/runtime event patterns instead of inventing a parallel observability path.
- Treat current dirty worktree files outside task 05 surfaces as read-only unless task execution proves they are directly required.
- Emit reusable-agent observability through the existing runtime event journal as a dedicated `reusable_agent.lifecycle` event kind rather than introducing a separate logging channel.
- Standardize nested blocked failures on the vocabulary `depth-limit`, `cycle-detected`, `access-denied`, `invalid-agent`, and `invalid-mcp`, and reuse that classification in CLI-facing error translation.
- Keep CLI integration tests on the `internal/core/run` facade by forwarding the ACP client swap hook there instead of importing `internal/core/run/internal/acpshared` from `internal/cli`.

## Learnings
- Shared workflow memory confirms agent-backed ACP sessions already carry resolved MCP servers on both `session/new` and `session/load`; task 05 should harden observability and safeguards on top of that seam, not replace it.
- The strongest pre-change signal is still the pending task file state: task 05 is `pending` and all subtasks/tests are unchecked.
- Targeted reusable-agent runtime coverage across `internal/core/agents`, `internal/core/agents/mcpserver`, `internal/core/run/internal/acpshared`, `internal/core/run/exec`, and `internal/core/run/executor` is `81.1%`.
- Full repository verification passed after the new observability and integration coverage landed.

## Files / Surfaces
- `pkg/compozy/events/event.go`
- `pkg/compozy/events/kinds/reusable_agent.go`
- `pkg/compozy/events/event_test.go`
- `pkg/compozy/events/docs_test.go`
- `docs/events.md`
- `internal/core/agents/reasons.go`
- `internal/core/agents/session_mcp.go`
- `internal/core/agents/session_mcp_test.go`
- `internal/core/agents/mcpserver/engine.go`
- `internal/core/agents/mcpserver/engine_test.go`
- `internal/core/agents/mcpserver/server_test.go`
- `internal/core/run/internal/acpshared/session_handler.go`
- `internal/core/run/internal/acpshared/reusable_agent_lifecycle.go`
- `internal/core/run/internal/acpshared/session_handler_test.go`
- `internal/core/run/internal/acpshared/command_io.go`
- `internal/core/run/internal/acpshared/command_io_test.go`
- `internal/core/run/internal/runshared/config.go`
- `internal/core/run/exec/exec.go`
- `internal/core/run/executor/execution_acp_integration_test.go`
- `internal/core/run/test_hooks.go`
- `internal/cli/root_command_execution_test.go`
- `internal/cli/run.go`

## Errors / Corrections
- CLI integration tests initially failed to compile because `internal/cli` imported `internal/core/run/internal/acpshared`; corrected by exposing the ACP client swap hook through `internal/core/run/test_hooks.go`.
- `make verify` initially failed on two `rangeValCopy` lint findings and one unused helper; corrected by switching to index iteration and removing the dead clone helper.

## Ready for Next Run
- Task 05 is complete. The next run can move to task 06 documentation using the landed lifecycle events, blocked reasons, and integration coverage as the source of truth.
