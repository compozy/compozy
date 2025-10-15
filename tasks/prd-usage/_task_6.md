## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>docs</domain>
<type>documentation</type>
<scope>documentation</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 6.0: Documentation, Schema & Examples

## Overview

Update API/CLI docs, add monitoring how-to, publish schema reference, regenerate OpenAPI outputs, and produce runnable examples demonstrating execution usage reporting. Include changelog entry as part of change control.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Update docs listed in `_docs.md` (API pages, CLI doc, monitoring guide, observability concept page, navigation config).
- Generate `schemas/execution-usage.json` and corresponding reference page.
- Regenerate OpenAPI/Swagger assets so `usage` object appears in API documentation.
- Create/refresh example projects under `examples/usage-reporting/*` with README and scripts.
- Add changelog entry and link ADR if required.
</requirements>

## Subtasks

- [ ] 6.1 Update API docs (`docs/api/executions.mdx`, `docs/api/agents.mdx`, `docs/api/tasks.mdx`) with usage examples
- [ ] 6.2 Add monitoring how-to and observability cross-links
- [ ] 6.3 Generate `schemas/execution-usage.json` + reference page and update `docs/source.config.ts`
- [ ] 6.4 Update CLI doc outputs and golden snippets
- [ ] 6.5 Build example projects (`workflow-summary`, `task-direct-exec`, `agent-sync`) with README + scripts
- [ ] 6.6 Regenerate OpenAPI/Swagger assets and run docs build (`npm run docs:build` or equivalent)
- [ ] 6.7 Add changelog entry (`feat(engine): add llm usage reporting pipeline`) and cite ADR

## Implementation Details

- Follow `_docs.md` and `_examples.md` outlines for content structure.
- Ensure example scripts rely on non-secret environment variables and document setup.
- Use API response example from `_techspec.md` to keep docs consistent with tests.

### Relevant Files

- `docs/api/*.mdx`
- `docs/how-to/monitor-usage.mdx`
- `docs/cli/executions.mdx`
- `docs/concepts/observability.mdx`
- `docs/reference/schemas/execution-usage.mdx`
- `docs/source.config.ts`
- `schemas/execution-usage.json`
- `examples/usage-reporting/*`
- `CHANGELOG.md`

### Dependent Files

- API & CLI implementations from Tasks 3.0 and 4.0
- Monitoring assets from Task 5.0

## Deliverables

- Documentation updates merged and docs build passing locally
- Schema file committed and referenced by docs site
- OpenAPI artifacts regenerated with `usage` object schemas
- Three runnable examples with README instructions
- Changelog entry referencing ADR and PR

## Tests

- Documentation acceptance from `_docs.md` and `_tests.md`:
  - [ ] Run docs build (`npm run docs:build` or equivalent) without errors
  - [ ] Execute example scripts/commands to confirm outputs match docs

## Success Criteria

- Docs site reflects new usage reporting endpoints and monitoring guidance
- Examples provide reproducible walkthroughs showing `usage` object
- Schema and navigation updates verified by docs build
- Changelog entry recorded for release notes
