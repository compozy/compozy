---
status: completed
parallelizable: true
blocked_by: ["1.0"]
---

<task_context>
<domain>schemas</domain>
<type>implementation|testing</type>
<scope>configuration</scope>
<complexity>low</complexity>
<dependencies></dependencies>
<unblocks>"10.0","13.0"</unblocks>
</task_context>

# Task 8.0: JSON Schemas

## Overview

Add embedder, vector DB, knowledge base, and knowledge binding JSON Schemas and wire them into project/workflow schemas.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Schema keys must mirror config structs; include defaults where applicable.
- Update project/workflow schemas to reference new definitions.
- Optional: minimal loader test if pattern exists in repo.
- Run `make fmt && make lint && make test` before completion.
</requirements>

## Subtasks

- [x] 8.1 Add `schemas/{embedder,vectordb,knowledge-base,knowledge-binding}.json`
- [x] 8.2 Update `schemas/project.json` and `schemas/workflow.json`
- [x] 8.3 Unit test for schema loading (if applicable)

## Sequencing

- Blocked by: 1.0
- Unblocks: 10.0, 13.0
- Parallelizable: Yes

## Implementation Details

Keep shapes aligned to `_techspec.md` YAML examples; avoid over‑specifying optional fields in MVP.

### Relevant Files

- `schemas/*.json`

### Dependent Files

- `docs/content/docs/schema/*`

## Success Criteria

- Schemas validate and integrate into docs build; tests pass.
