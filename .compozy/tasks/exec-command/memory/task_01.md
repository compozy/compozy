# Task Memory: task_01.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Add the shared runtime/config surface for `exec` only: execution mode, output format, prompt-source metadata, `[exec]` workspace defaults, and validation/tests.

## Important Decisions
- Treat `_techspec.md` plus ADR-001 and ADR-003 as the approved design; do not add the `exec` Cobra command in this task.
- Reuse the existing CLI workspace merge path by extending command-kind/config branching instead of creating a parallel config resolver.
- Keep `output_format=json` and prompt-source fields valid only for `exec` mode; other modes stay on text output and reject ad hoc prompt-source config.

## Learnings
- Current runtime validation only accepts `pr-review` and `prd-tasks`.
- Workspace config currently supports `[defaults]`, `[start]`, `[tasks]`, `[fix_reviews]`, and `[fetch_reviews]`; there is no `[exec]` section yet.
- Two unrelated CLI tests were brittle against current repo state:
  - one assumed the committed `acp-integration` fixture still lived under active tasks instead of `_archived`
  - one resolved `testdata/start_help.golden` relative to process cwd instead of the package directory

## Files / Surfaces
- `internal/core/model/model.go`
- `internal/core/api.go`
- `internal/core/workspace/config.go`
- `internal/cli/workspace_config.go`
- `internal/core/agent/registry.go`
- `internal/core/model/model_test.go`
- `internal/core/workspace/config_test.go`
- `internal/core/agent/registry_test.go`
- `internal/cli/workspace_config_test.go`
- `internal/cli/root_test.go`
- `internal/cli/migrate_command_test.go`

## Errors / Corrections
- Skill paths in the prompt examples resolved to repository-local `.agents/skills/...`; loaded those actual files before editing.
- Refactored `agent.ValidateRuntimeConfig` into helper functions after `make verify` failed the repo’s `gocyclo` lint gate.

## Ready for Next Run
- Task 01 is implemented and verified. Next run can start from task 02 or task 03 using the shared `exec` runtime/config surface and the recorded fixture-path/testdata learnings above.
