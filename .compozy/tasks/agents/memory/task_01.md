# Task Memory: task_01.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Build `internal/core/agents` for workspace/global discovery, `AGENT.md` parsing, `mcp.json` loading, validation, and whole-directory override resolution, with tests and `make verify` as the completion gate.

## Important Decisions
- Discovery returns a `Catalog` with `Agents` plus `Problems` so malformed agents surface without corrupting valid resolution.
- Workspace/global conflicts are resolved before parsing, so a workspace directory shadows the entire global directory even when the workspace copy is invalid.
- `mcp.json` is normalized during load by expanding `${VAR}` placeholders and resolving relative commands against the agent directory.

## Learnings
- The repository already has reusable YAML frontmatter parsing in `internal/core/frontmatter`, but unsupported agent metadata fields need an explicit raw-map validation pass because unknown YAML keys are otherwise ignored.
- Current runtime validation rules for `ide`, `reasoning_effort`, and `access_mode` already exist across `internal/core/agent` and `internal/core/model`, so the new package can stay aligned with existing runtime conventions.
- Targeted validation currently passes: `go test ./internal/core/agents -count=1 -coverprofile=/tmp/agents.cover && go tool cover -func=/tmp/agents.cover` with `84.8%` package coverage.
- Repository-wide verification also passed after the final lint cleanup: `make verify` finished with `0 issues`, `DONE 1134 tests`, and a successful `go build`.

## Files / Surfaces
- `internal/core/agents/agents.go`
- `internal/core/agents/agents_test.go`
- `.codex/ledger/2026-04-10-MEMORY-agent-registry.md`

## Errors / Corrections
- Initial package-local test run failed because `internal/core/agents/agents_test.go` was missing the `internal/core/model` import used by test helpers; fixed and reran the package tests successfully.
- The first `make verify` run failed in lint on a large range-value copy in `Catalog.Resolve` and an ineffectual `body` assignment in `parseAgentDefinition`; both were corrected before the final successful verification run.

## Ready for Next Run
- Update `task_01.md` and `_tasks.md`, then create the required local commit. Full verification is already green.
