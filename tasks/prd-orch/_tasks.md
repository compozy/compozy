# Orchestrator FSM — Implementation Task Summary

## Relevant Files

### Core Implementation Files

- `engine/llm/orchestrator/state_machine.go` - FSM builder (`newLoopFSM`) and state/event IDs
- `engine/llm/orchestrator/loop.go` - Replace for-loop with FSM-driven execution
- `engine/llm/orchestrator/orchestrator.go` - Wire FSM into entry points and outputs
- `engine/llm/orchestrator/llm_invoker.go` - LLM invocation from `await_llm`
- `engine/llm/orchestrator/tool_executor.go` - Tool execution from `process_tools`
- `engine/llm/orchestrator/response_handler.go` - Completion handling from `handle_completion`
- `engine/llm/orchestrator/memory.go` - Finalization and memory persistence

### Integration Points

- `engine/llm/orchestrator/request_builder.go` - Request assembly; ensure compatibility
- `engine/llm/orchestrator/fingerprint.go` - Progress/no-progress detection used by guards
- `engine/llm/orchestrator/loop_state.go` - Budgets and counters referenced by guards
- `pkg/config` - Read config via `config.FromContext(ctx)`; no feature flags
- `logger` - Use `logger.FromContext(ctx)` for state/event logs

### Documentation Files

- `tasks/prd-orch/_prd.md` - Product Requirements (greenfield, no feature flags)
- `tasks/prd-orch/_techspec.md` - Technical Specification (FSM design details)
- `docs/orchestrator-fsm.md` - State diagram and developer notes (to add)

## Tasks

- [x] 1.0 Create FSM scaffolding and builder (`state_machine.go`)
- [x] 2.0 Replace legacy loop with FSM in `conversationLoop.Run`
- [x] 3.0 Implement `await_llm` state and event plumbing
- [x] 4.0 Implement `evaluate_response` branching (no_tool vs tool_calls)
- [x] 5.0 Implement `process_tools` with concurrent execution
- [x] 6.0 Implement `update_budgets` guards (budgets, no-progress)
- [x] 7.0 Implement `handle_completion` (retry/success) mapping
- [x] 8.0 Implement `finalize` (memory persist, output packaging)
- [ ] 9.0 Centralize logging/metrics per transition
- [ ] 10.0 Remove feature flag/legacy path and update tests/docs

## Execution Plan

- Critical Path: 1.0 → 2.0 → (3.0, 4.0, 5.0 in parallel) → 6.0 → 7.0 → 8.0 → 9.0 → 10.0
- Parallel Track A: 3.0, 4.0, 5.0 (independent once 1.0 and 2.0 scaffolding are in place)
- Parallel Track B: 9.0 can start after 2.0 to add instrumentation hooks

Notes:

- Keep `make lint` and `make test` green at every step.
- Use `logger.FromContext(ctx)` and `config.FromContext(ctx)` everywhere; no globals and no feature flags.
- Maintain external behavior; internal loop implementation is fully replaced (greenfield).
