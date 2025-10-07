---
status: pending
parallelizable: true
blocked_by: ["6.0", "7.0", "8.0", "10.0"]
---

<task_context>
<domain>docs</domain>
<type>documentation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies></dependencies>
<unblocks></unblocks>
</task_context>

# Task 11.0: Documentation and examples

## Overview

Document the builtin usage, input schema, examples (prompt vs. structured plan), limits, and troubleshooting. Provide example workflow YAML and agent tool‑calling snippets.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- User guide under `docs/` and example in `examples/` folder.
- Include copy‑paste examples matching PRD user stories.
</requirements>

## Subtasks

- [ ] 11.1 User guide
- [ ] 11.2 Example workflow and agent snippets

## Sequencing

- Blocked by: 6.0, 7.0, 8.0, 10.0
- Unblocks: —
- Parallelizable: Yes

## Success Criteria

- Examples run locally; docs reviewed and approved
