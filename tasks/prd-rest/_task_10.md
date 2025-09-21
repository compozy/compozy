---
status: pending
parallelizable: true
blocked_by: ["6.0", "7.0", "8.0", "9.0"]
---

<task_context>
<domain>project/docs</domain>
<type>documentation</type>
<scope>cleanup|rollout</scope>
<complexity>low</complexity>
<dependencies>http_server</dependencies>
<unblocks>"—"</unblocks>
</task_context>

# Task 10.0: Rollout & Cleanup

## Overview

Finalize removal of legacy surfaces, update examples/docs, and run an org‑wide scan for `/resources` usage.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Delete unused `engine/resources/uc/*` entries no longer referenced.
- Update docs and examples to only reference resource‑specific endpoints.
- Org‑wide code search for `/resources` and attach report to PR.
</requirements>

## Subtasks

- [ ] 10.1 Remove dead UC code and references.
- [ ] 10.2 Update docs/examples/CLI to new endpoints.
- [ ] 10.3 Org‑wide `/resources` usage audit and attach results.

## Sequencing

- Blocked by: 6.0, 7.0, 8.0, 9.0
- Unblocks: —
- Parallelizable: Yes (docs and audit)

## Implementation Details

Coordinate with maintainers for removal PRs and docs site updates.

### Relevant Files

- `engine/resources/uc/*`
- `docs/**`

### Dependent Files

- `tasks/prd-rest/_prd.md`

## Success Criteria

- No legacy `/resources` paths in code/docs; PR includes audit results.
