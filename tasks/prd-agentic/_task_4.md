---
status: pending
parallelizable: true
blocked_by: ["3.0"]
---

<task_context>
<domain>engine/tool/builtin/orchestrate/planner</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies></dependencies>
<unblocks>"6.0"</unblocks>
</task_context>

# Task 4.0: Implement planner (prompt → plan) with guardrails

## Overview

Implement a planner that converts natural‑language prompts into structured `Plan` JSON. Use a constrained prompt and validate output against schema. Add recursion/safety flags to prevent self‑invocation loops.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Deterministic system prompt; reject non‑conforming responses.
- Optional planner disable flag; rely on structured input path when disabled.
- Tests with dynamic mock LLM adapter to simulate planning outputs and failures.
</requirements>

## Subtasks

- [ ] 4.1 Planner scaffold and prompt
- [ ] 4.2 Validation and failure modes
- [ ] 4.3 Tests with mock LLM

## Sequencing

- Blocked by: 3.0
- Unblocks: 6.0
- Parallelizable: Yes

## Implementation Details

Integrate via `ExecuteTask`/LLM service when called from builtin; do not expose externally.

### Relevant Files

- `engine/llm/service.go`

### Dependent Files

- `engine/tool/builtin/orchestrate/handler.go`

## Success Criteria

- Planner produces valid plans; failure cases handled gracefully
