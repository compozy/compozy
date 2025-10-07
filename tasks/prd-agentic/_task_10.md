---
status: pending
parallelizable: false
blocked_by: ["5.0", "6.0", "7.0", "8.0", "9.0"]
---

<task_context>
<domain>engine/tool/builtin/orchestrate</domain>
<type>testing</type>
<scope>quality</scope>
<complexity>medium</complexity>
<dependencies>temporal</dependencies>
<unblocks>"11.0"</unblocks>
</task_context>

# Task 10.0: Tests — unit, integration, and load caps

## Overview

Add unit tests for plan validation, planner outputs, executor sequencing/parallel, and handler end‑to‑end using dynamic mock LLM and in‑memory repos.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Unit tests for each package; integration test via `llm.Service` tool calling.
- Benchmark/short load test for concurrency caps and cancellation.
</requirements>

## Subtasks

- [ ] 10.1 Unit tests (planner/executor/handler)
- [ ] 10.2 Integration test (registry + service)
- [ ] 10.3 Benchmarks for caps

## Sequencing

- Blocked by: 5.0, 6.0, 7.0, 8.0, 9.0
- Unblocks: 11.0
- Parallelizable: No

## Success Criteria

- All new tests pass and are deterministic
