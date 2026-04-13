# Task Memory: task_04.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Completed the reusable-agent CLI surface for task 04: `compozy exec --agent <name>`, `compozy agents list`, `compozy agents inspect`, and hidden `compozy mcp-serve`.
- Verified the implementation with focused race coverage and a clean `make verify`.

## Important Decisions
- Kept the CLI layer thin by resolving reusable agents through `internal/core/agents.ResolveExecutionContext` and existing discovery APIs instead of creating CLI-specific resolution logic.
- Implemented the hidden MCP host as `compozy mcp-serve --server compozy`, backed by a small wrapper around the official Go MCP SDK in `internal/core/agents/mcpserver/server.go`.
- Preserved runtime precedence by threading the selected agent name through existing command state/runtime config and letting explicit CLI flags continue to override agent defaults.

## Learnings
- `exec` persisted-mode integration tests that swap process `os.Stdout` / `os.Stderr` cannot run as top-level parallel tests in `internal/cli`; they race with Cobra help/output lookups across the package under `-race`.
- `agents inspect` can report invalid frontmatter or `mcp.json` details without blocking on discovery because the registry already returns both resolved agents and validation problems.

## Files / Surfaces
- `internal/cli/commands.go`
- `internal/cli/root.go`
- `internal/cli/run.go`
- `internal/cli/state.go`
- `internal/cli/agents_commands.go`
- `internal/cli/agents_commands_test.go`
- `internal/cli/root_test.go`
- `internal/cli/root_command_execution_test.go`
- `internal/cli/testdata/exec_help.golden`
- `internal/core/agents/mcpserver/server.go`
- `internal/core/agents/mcpserver/server_test.go`
- `go.mod`
- `go.sum`

## Errors / Corrections
- Initial `make verify` failed because the new persisted exec tests were marked `t.Parallel()` while using `executeRootCommandCapturingProcessIO`, which mutates process-wide stdio handles.
- Correction: serialized only those process-stdio tests by removing top-level parallelization, then re-ran `go test ./internal/cli -race -count=1` and `make verify` successfully.

## Ready for Next Run
- Task tracking still needs the final local commit once the diff review is complete.
