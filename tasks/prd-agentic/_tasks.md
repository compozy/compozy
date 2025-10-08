# Agentic Built-in Tool (cp\_\_agent_orchestrate) — Implementation Task Summary

## Relevant Files

### Core Implementation Files

- `engine/tool/builtin` — Built-in tool framework (definitions, registry, telemetry)
- `engine/tool/native/catalog.go` — Native builtin registration catalog
- `engine/llm/service.go` — Tool registry wiring inside LLM service
- `engine/llm/orchestrator/tool_executor.go` — Tool execution, concurrency, logging
- `engine/task/router/direct_executor.go` — Sync execution pipeline (DirectExecutor)
- `engine/agent/router/exec.go` — Agent sync endpoints using DirectExecutor
- `engine/task/uc/exec_task.go` — ExecuteTask orchestrates agent/tool/direct LLM
- `engine/resources` — ResourceStore APIs for agents/tools

### Integration Points

- `engine/infra/server/appstate` — App state/context sources
- `engine/tool/resolver` — Tool inheritance & resolution
- `pkg/config` / `pkg/logger` — Config/log context propagation

### Documentation Files

- `tasks/prd-agentic/_prd.md` — Product Requirements
- `tasks/prd-agentic/_techspec.md` — Technical Specification

## Tasks

- [x] 1.0 Extract reusable Agent Runner service from router — size: medium (not batchable)
- [x] 2.0 Add tool context bridge for appstate/repo propagation — size: medium (not batchable)
- [x] 3.0 Define orchestration plan model and schema — size: small (batchable)
- [x] 4.0 Implement planner (prompt → plan) with guardrails — size: medium (not batchable)
- [x] 5.0 Implement executor (sequential + parallel) with limits — size: high (not batchable)
- [x] 6.0 Implement cp\_\_agent_orchestrate builtin handler — size: medium (not batchable)
- [x] 7.0 Register builtin in native catalog and service wiring — size: small (batchable)
- [x] 8.0 Config: runtime.native_tools.agent_orchestrator limits — size: small (batchable)
- [x] 9.0 Telemetry/metrics and logging for steps and totals — size: small (batchable)
- [ ] 10.0 Tests: unit + integration + benchmarks (caps) — size: medium (not batchable)
- [ ] 11.0 Documentation and examples — size: small (batchable)

## Execution Plan

- Critical Path: 1.0 → 2.0 → 3.0 → 5.0 → 6.0 → 7.0 → 8.0 → 9.0 → 10.0 → 11.0
- Parallel Track A: 4.0 (planner) can proceed after 3.0; integrates at 6.0
- Parallel Track B: 11.0 docs may start after 3.0 with updates after 6.0

## Batch Candidates (one-commit bundles once prerequisites met)

- Batch A (post-1.0): 3.0 Define plan model and schema
  - Rationale: pure types + schema + validators; isolated from runtime.
  - Tasks: 3.0

- Batch B (post-6.0): 8.0 Config limits + 9.0 Telemetry/metrics
  - Rationale: both are small wiring changes; 9.0 depends on 8.0 and both depend on 6.0. Safe to land together as “runtime knobs + instrumentation”.
  - Tasks: 8.0, 9.0

- Batch C (post-6.0): 7.0 Catalog registration
  - Rationale: trivial registration; can be included with Batch B if preferred.
  - Tasks: 7.0

- Batch D (finalization, post-10.0): 11.0 Documentation and examples
  - Rationale: small, independent content; land as a single docs commit after tests stabilize.
  - Tasks: 11.0

Notes

- Medium/high tasks (1.0, 2.0, 4.0, 5.0, 6.0, 10.0) should land separately to simplify review and rollback.

## Latest Progress (October 8, 2025)

- Completed Task 5.0 by introducing the orchestrate executor (`engine/tool/builtin/orchestrate/executor.go`) and FSM (`engine/tool/builtin/orchestrate/fsm.go`) that sequence/parallelize plan steps, enforce depth/step/parallel limits, and propagate per-step results.
- Added executor unit tests (`engine/tool/builtin/orchestrate/executor_test.go`, `engine/tool/builtin/orchestrate/fsm_test.go`) covering sequential flow, bounded parallel fan-out, cancellations, and failure result propagation.
- Extended tool context helpers (`engine/tool/context/context.go`) with orchestrator depth tracking, enabling recursive-limit enforcement across nested invocations.
- Completed Task 7.0 by registering `cp__agent_orchestrate` through the native catalog (`engine/tool/native/catalog.go`), wiring `engine/llm/service.go` to load environment-aware definitions, and extending handler tests to assert discovery via `native.Definitions`.
