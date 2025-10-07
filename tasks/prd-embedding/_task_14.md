---
status: pending
parallelizable: true
blocked_by: ["10.0","11.0","3.0"]
---

<task_context>
<domain>examples</domain>
<type>implementation</type>
<scope>documentation|configuration</scope>
<complexity>low</complexity>
<dependencies>database|http_server</dependencies>
<unblocks>"15.0"</unblocks>
</task_context>

# Task 14.0: Examples Implementation

## Overview
Create P0 example folders (quickstart-markdown-glob, pgvector-basic, pdf-url, query-cli) with README runbooks per `_examples.md`.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Keep artifacts ≤100KB; require env interpolation for secrets; callable via CLI.
- Ensure each README includes exact commands and expected outputs.
- Validate examples against schemas and runbooks.
</requirements>

## Subtasks
- [ ] 14.1 Add quickstart-markdown-glob
- [ ] 14.2 Add pgvector-basic
- [ ] 14.3 Add pdf-url
- [ ] 14.4 Add query-cli

## Sequencing
- Blocked by: 10.0, 11.0, 3.0
- Unblocks: 15.0
- Parallelizable: Yes

## Implementation Details
Use `make start-docker` for pgvector where needed; prefer in‑memory for quickstart if implemented.

### Relevant Files
- `examples/knowledge/*`

### Dependent Files
- Integration tests and docs walkthroughs

## Success Criteria
- All P0 examples runnable per README; outputs match expectations.
