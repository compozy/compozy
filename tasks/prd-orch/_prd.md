# Product Requirements Document (PRD): LLM Orchestrator FSM Integration

## Overview

Implement an explicit finite state machine (FSM) to drive the LLM orchestration loop. Today, transitions (invoke LLM → inspect response → execute tools → apply budgets → complete or retry) are embedded across multiple files, making behavior harder to reason about, extend, and instrument. The FSM centralizes allowed transitions, guard checks, and lifecycle instrumentation while preserving current external behavior (budgets, retries, structured output). Primary users are platform engineers, orchestrator maintainers, and SREs who need predictable execution flow and consistent telemetry.

## Goals

- Centralize loop control as a state machine with explicit states and events
- Preserve existing behavior (budgets, retries, structured output) with no feature flags (greenfield replacement)
- Reduce branching and scattered logic; improve maintainability and testability
- Standardize logging and metrics per transition using `logger.FromContext(ctx)`
- Enforce context/config patterns via `config.FromContext(ctx)`; no global singletons
- Measurable targets:
  - ≤ +2% p95 iteration duration compared to legacy loop (same workload)
  - 100% transitions produce structured logs with state/event labels
  - 80%+ unit test coverage for FSM states, guards, and transitions
  - Zero regressions in existing orchestrator tests

## User Stories

- As a platform engineer, I want a clear state diagram so I can add new phases without touching unrelated logic.
- As an orchestrator maintainer, I want budget/progress checks expressed as guards so policies live in one place.
- As an SRE, I want state/event labels in logs/metrics so I can trace and alert on stuck or failing transitions.
- As a feature developer, I want a single canonical FSM implementation (no toggle) to reduce operational complexity.

## Core Features

High‑level features:

- Explicit states: init, await_llm, evaluate_response, process_tools, update_budgets, handle_completion, finalize, terminate_error
- Explicit events: start_loop, llm_response, response_no_tool, response_with_tools, tools_executed, budget_ok, budget_exceeded, completion_retry, completion_success, failure
- Guard functions for budgets, progress detection, and structured retries
- Callback layout using `enter_/leave_/before_/after_` hooks to encapsulate side effects and instrumentation
- Centralized telemetry per transition

Functional requirements (numbered):

1. The system must construct an FSM with the states and events defined above.
2. The system must trigger `start_loop` once per action and continue until a terminal state.
3. The system must invoke the LLM in `enter_await_llm` and carry the response into `llm_response`.
4. The system must route `evaluate_response` to `response_no_tool` or `response_with_tools` based on tool call presence.
5. The system must execute tool calls concurrently in `enter_process_tools` and attach results to metadata.
6. The system must evaluate budget/progress guards before returning to `await_llm`.
7. The system must handle completion without tools in `enter_handle_completion`, emitting `completion_retry` or `completion_success`.
8. The system must persist memories and package output on `enter_finalize`.
9. The system must propagate any fatal error through a unified `failure` event to `terminate_error`.
10. The system must log state/event, iteration counters, and guard outcomes using `logger.FromContext(ctx)`.
11. The system must read configuration (limits only; no feature flags) via `config.FromContext(ctx)` in all paths.
12. The system must replace the legacy loop entirely (no dual-path or feature flag).

## User Experience

This is an internal, developer-facing experience. UX manifests as:

- Clear state/event logs and metrics for each transition
- Consistent error propagation and reasons when transitions fail
- Documentation with a state diagram and examples of typical traces

## High-Level Technical Constraints

- Use `github.com/looplab/fsm` for FSM implementation
- Strict context propagation; never use `context.Background()` in runtime paths
- Use `logger.FromContext(ctx)` for all logging and `config.FromContext(ctx)` for configuration
- No global configuration singletons
- Maintain performance overhead to ≤ +2% p95 iteration duration vs. baseline
- Preserve current structured output and retry semantics
- No conversational clarification state in this iteration

## Non-Goals (Out of Scope)

- Introducing a conversational clarification state or interactive UX
- Streaming tool execution or asynchronous transitions (may be future work)
- Changing validator/structured output semantics beyond moving logic into callbacks/guards

## Phased Rollout Plan

Greenfield cutover (no feature flags):

- Phase 1: FSM scaffold, loop refactor, tool execution port, budget/progress guards, completion handling
- Phase 2: Finalize output & memory sync validation, unified failure event usage across all error sites, wider test coverage
- Phase 3: Observability polish and state metrics; remove any dead code from the legacy loop (fully replaced in Phase 1)

## Success Metrics

- Stability: zero new orchestrator regressions in CI
- Performance: ≤ +2% p95 loop duration delta vs. legacy under representative workloads
- Coverage: ≥ 80% unit coverage over FSM guards and transitions
- Operability: state/event present in 100% orchestrator loop logs; dashboards show per‑state timings

## Risks and Mitigations

- Increased complexity from abstraction → Mitigate with diagrams and focused documentation
- Guard regressions changing behavior → Mitigate with unit tests on thresholds and golden tests for outputs
- Third‑party dependency drift → Pin module version; review changes before upgrades

## Open Questions

- Should we add asynchronous transitions in a follow‑up to improve streaming behavior?
- What additional metrics are most valuable to surface per state (e.g., durations, error rates)?
- Are there any external consumers relying on internal loop timing that require explicit documentation during cutover?

## Greenfield Approach (No Backwards Compatibility)

- Replace the legacy loop completely; do not ship or maintain a dual path.
- Do not introduce feature flags or toggles; the FSM is the only runtime path.
- Maintain external behavior and public contracts; tests and lints must remain green.
- Update documentation and developer guides to reflect the single-path FSM design.

## Appendix

- Tech Spec: `tasks/prd-orch/_techspec.md`
- Library reference: `github.com/looplab/fsm`
