status: completed
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

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Unit tests for each package; integration test via `llm.Service` tool calling.
- Benchmark/short load test for concurrency caps and cancellation.
</requirements>

## Subtasks

- [x] 10.1 Unit tests (planner/executor/handler)
- [x] 10.2 Integration test (registry + service)
- [x] 10.3 Benchmarks for caps

## Sequencing

- Blocked by: 5.0, 6.0, 7.0, 8.0, 9.0
- Unblocks: 11.0
- Parallelizable: No

## Success Criteria

- All new tests pass and are deterministic

## Outcome

- Validated existing unit suites across planner, executor, and handler packages to ensure coverage for schema validation, concurrency limits, and handler error paths continues to pass.
- Added `test/integration/tool/orchestrate_integration_test.go`, exercising `llm.Service` with a scripted LLM client to drive `cp__agent_orchestrate` end-to-end and assert step outputs/bindings.
- Confirmed benchmark coverage for fan-out and cancellation scenarios (`engine/tool/builtin/orchestrate/executor_test.go`, `engine/tool/builtin/orchestrate/plan_test.go`) remains in place as load caps regressions.
- `make lint` and `make test` succeed after the new test additions.
