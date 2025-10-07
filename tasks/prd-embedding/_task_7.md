---
status: pending
parallelizable: true
blocked_by: ["1.0"]
---

<task_context>
<domain>engine/project|engine/workflow|engine/task|engine/agent</domain>
<type>implementation|testing</type>
<scope>configuration</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
<unblocks>"6.0","8.0","9.0"</unblocks>
</task_context>

# Task 7.0: YAML & Binding Resolution

## Overview
Extend project/workflow/task/agent config to support knowledge bindings with MVP single‑binding cardinality and deterministic precedence (workflow → project → inline overrides).

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add binding structs and parsing; integrate with autoload.
- Enforce single binding per workflow in MVP; clear error on multiple.
- Unit tests for precedence and decode via mapstructure.
- Run `make fmt && make lint && make test` before completion.
</requirements>

## Subtasks
- [ ] 7.1 Implement binding structs and parsing
- [ ] 7.2 Unit tests `engine/knowledge/service_test.go` (precedence)
- [ ] 7.3 Unit tests `engine/project/config_test.go` (decode arrays)

## Sequencing
- Blocked by: 1.0
- Unblocks: 6.0, 8.0, 9.0
- Parallelizable: Yes

## Implementation Details
Bindings must be available to `engine/knowledge/service` and orchestrator. Keep defaults aligned with `_techspec.md`.

### Relevant Files
- `engine/project/*`
- `engine/workflow/*`

### Dependent Files
- `engine/knowledge/service.go`

## Success Criteria
- Binding resolution and precedence validated by unit tests.
