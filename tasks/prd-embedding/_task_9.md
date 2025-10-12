---
status: completed
parallelizable: false
blocked_by: ["6.0", "7.0"]
---

<task_context>
<domain>engine/llm/orchestrator</domain>
<type>integration|testing</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>external_apis</dependencies>
<unblocks>"15.0"</unblocks>
</task_context>

# Task 9.0: LLM Orchestrator Integration

## Overview

Inject retrieved knowledge chunks into prompts prior to model invocation with deterministic ordering and token budgeting.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Use `logger.FromContext(ctx)` / `config.FromContext(ctx)` throughout; no globals.
- Ensure injection runs before tools and after memory per orchestrator rules.
- Unit tests with stubbed LLM adapter to verify injection ordering and budgeting.
- Run `make fmt && make lint && make test` before completion.
</requirements>

## Subtasks

- [x] 9.1 Wire `engine/knowledge/service` into orchestrator assembly
- [x] 9.2 Add unit tests (stub adapter) for injection order and budgeting

## Sequencing

- Blocked by: 6.0, 7.0
- Unblocks: 15.0
- Parallelizable: No

## Implementation Details

Follow `_techspec.md` injection notes; keep payload shape inline text only.

### Relevant Files

- `engine/llm/orchestrator/*`

### Dependent Files

- `test/integration/knowledge/workflow_binding_test.go`

## Success Criteria

- Injection works and tests pass; context usage verified.
