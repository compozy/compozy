## markdown

## status: pending

<task_context>
<domain>engine/infra/server/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 11.0: Remove `fields=` Feature Project‑Wide (Greenfield Step 0)

## Overview

Permanently remove the `fields=` sparse fieldset feature from the API. Delete helpers, strip Swagger parameters, and update all handlers and tests. This is a mandatory pre‑work step before introducing typed DTO responses.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Remove helper functions and all references to `fields=`:
  - Delete usages of `ParseFieldsQuery` and `FilterMapFields`.
  - Remove `@Param fields` from Swagger annotations.
- Keep `expand=` semantics intact where present.
- Update tests that assert on `fields=` behavior.
- Regenerate Swagger and ensure no `fields` params remain.
</requirements>

## Files To Change (explicit)

Helpers (remove functions and related code):

- `engine/infra/server/router/query.go` (remove `ParseFieldsQuery`, `FilterMapFields`; keep `ParseExpandQuery`).

Routers (remove `@Param fields` and all code using `fields`):

- `engine/agent/router/agents_top.go`
- `engine/model/router/models.go`
- `engine/memoryconfig/router/memories.go`
- `engine/project/router/project.go`
- `engine/schema/router/schemas.go`
- `engine/task/router/tasks_top.go`
- `engine/tool/router/tools_top.go`
- `engine/workflow/router/workflows.go`

Tests (remove or rewrite assertions that use `?fields=`):

- `test/integration/resources/workflows_test.go` (compact view request with `fields=id,tasks,task_ids,task_count`)
- `test/integration/resources/tasks_test.go` (request to `/api/v0/tasks?fields=...`)
- `test/integration/resources/agents_test.go` (request to `/api/v0/agents?fields=...`)
- `test/integration/resources/project_test.go` (request to `/api/v0/project?fields=name`)

Docs (regenerate):

- `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` (generated) — ensure no `fields` params appear.

## Subtasks

- [ ] 11.1 Remove helpers and all imports/usages of `ParseFieldsQuery`/`FilterMapFields`.
- [ ] 11.2 Strip `@Param fields` from all affected routers; remove body filtering code.
- [ ] 11.3 Update tests to assert on typed shapes or default representations (keep ETag/Link checks).
- [ ] 11.4 Regenerate Swagger and verify removal of all `fields` parameters.

## Implementation Details

Remove codepaths uniformly; keep `expand=` intact; avoid leaving dead imports.

### Relevant Files

- `engine/infra/server/router/query.go`

### Dependent Files

- Routers and tests listed above

## Success Criteria

- No references to `fields` helpers or query params remain in code or Swagger comments.
- All affected handlers compile and run without map filtering logic.
- Tests pass without `fields` assertions.
