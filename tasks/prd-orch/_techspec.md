# LLM Orchestrator FSM Integration Plan

## Background

- Goal: reshape the LLM orchestrator loop to use an explicit finite state machine (FSM) so we can control tool invocation, result processing, and completion handling with well-defined transitions.
- Constraint: we do not expose a conversational clarification interface today, so the design must skip "question" states and operate entirely on action/execution/completion loops.
- Standards: changes must keep existing behavior intact (budgets, retries, structured output) while following project rules for context propagation, logging, and configuration access.

## External Research Summary

- [`looplab/fsm`] provides a Go-native FSM with declarative event descriptors (`Events`) and callback hooks (`before_`, `leave_`, `enter_`, `after_`) to inject behavior around transitions. Callbacks receive `context.Context` and an event envelope, matching our orchestrator requirement to inherit context and log via `logger.FromContext`.citeturn0fetch0
- The library exposes `FSM.Event(ctx, name, ...args)` for synchronous transitions, `FSM.Can` to probe available moves, metadata storage keyed on transitions, and a safe concurrency model via internal mutexes. This lets us gate transitions on dynamic guards such as error budgets.citeturn0fetch0
- The `looplab/fsm` public API is stable and available on pkg.go.dev, confirming continued maintenance (Go module path `github.com/looplab/fsm`) with examples demonstrating context-aware callbacks.citeturn0fetch1
- Community usage patterns emphasize guarding transitions with business rules and injecting side effects inside `enter_state`/`after_<event>` callbacks instead of duplicating logic in callers, aligning with our desire to centralize tool budget checks.citeturn0search3

> Context7 note: the Context7 catalog does not currently provide documentation for `github.com/looplab/fsm`; lookups for `/looplab/fsm` returned a "library does not exist" response, so we deferred to the upstream README and pkg.go.dev for authoritative references.

## Current Orchestrator Flow (Snapshot)

- `engine/llm/orchestrator/orchestrator.go:45-94` builds dependencies, creates `conversationLoop`, and enters `Run` with a mutable `loopState` that stores budgets and memory handles.
- `engine/llm/orchestrator/loop.go:35-95` performs a `for iter := 0; iter < maxIter; iter++` loop, invoking the LLM, branching on the presence of tool calls, and manually appending messages/results to the request.
- `engine/llm/orchestrator/tool_executor.go:62-153` executes tools concurrently then mutates `loopState` counters to track error budgets and progress fingerprints.
- `engine/llm/orchestrator/response_handler.go:34-145` handles completion attempts (JSON mode, output validation) and uses synthetic "tool" messages when the model fails validation.
- Progress/no-progress detection lives in `progress.go`, while retry budgets (structured output, validator) are implemented as increments in `loopState`.

### Observed Pain Points

1. Transition logic is scattered; understanding the allowed paths requires reading four files in tandem.
2. Budgets/guards are mutated in tool handlers instead of being first-class policies.
3. Extending the loop with new phases (e.g. streaming adapters, additional validation) risks more branching inside `Run`.
4. Logging/metrics happen in multiple places without a unified view of the orchestrator lifecycle.

## Target FSM Design

### Proposed States

| State ID            | Purpose                                           | Entry Actions                                   | Exit Criteria                                              |
| ------------------- | ------------------------------------------------- | ----------------------------------------------- | ---------------------------------------------------------- |
| `init`              | Prepare loop context, reset counters, load memory | initialize `LoopContext`, emit metrics          | always transitions to `await_llm`                          |
| `await_llm`         | Issue request to LLM client                       | call `invoker.Invoke`, log iteration context    | receives LLM response -> choose branch                     |
| `evaluate_response` | Inspect LLM response                              | categorize as `tool_calls`, `no_tool`, `error`  | branch via guards to next state                            |
| `process_tools`     | Execute tool calls concurrently                   | call tool executor, collect results             | completion when results available                          |
| `update_budgets`    | Apply success/error budgets, progress checks      | update counters, evaluate guard thresholds      | guard failure -> `terminate_error`; success -> `await_llm` |
| `handle_completion` | Run response handler for final output             | call `responses.HandleNoToolCalls`              | `cont` -> `await_llm`; success -> `finalize`               |
| `finalize`          | Persist memories, emit completion output          | call `memory.StoreAsync`, package `core.Output` | terminal                                                   |
| `terminate_error`   | Handle fatal failures (budgets, no-progress)      | record reason, propagate error                  | terminal                                                   |

### Transition Events

| Event Name            | Source → Destination                      | Guard Logic                                             | Notes                                   |
| --------------------- | ----------------------------------------- | ------------------------------------------------------- | --------------------------------------- |
| `start_loop`          | `init` → `await_llm`                      | always true                                             | triggered once per action               |
| `llm_response`        | `await_llm` → `evaluate_response`         | no guard                                                | includes response payload in event args |
| `response_no_tool`    | `evaluate_response` → `handle_completion` | `len(toolCalls)==0`                                     | retains current handler semantics       |
| `response_with_tools` | `evaluate_response` → `process_tools`     | `len(toolCalls)>0`                                      | carries tool call slice                 |
| `tools_executed`      | `process_tools` → `update_budgets`        | executor completed without fatal error                  | attaches results                        |
| `budget_ok`           | `update_budgets` → `await_llm`            | budgets below thresholds AND progress guard passes      | resets per-tool counters                |
| `budget_exceeded`     | `update_budgets` → `terminate_error`      | guard fails                                             | reuses existing error payloads          |
| `completion_retry`    | `handle_completion` → `await_llm`         | handler returned `cont=true`                            | increments structured retry counter     |
| `completion_success`  | `handle_completion` → `finalize`          | handler returned output                                 | stores output in FSM metadata           |
| `failure`             | any → `terminate_error`                   | invoked when invoker/executor/handler returns fatal err | ensures consistent fatal handling       |

### Context Object

- Rename `loopState` to `LoopContext` embedding:
  - Tool success/error counters and fingerprints
  - Structured retry counters for output/validator paths
  - Last LLM response + tool results (for telemetry)
  - Handles to `MemoryContext`, `Request`, `llmReq`
- Attach the context to FSM metadata (`fsm.SetMetadata("ctx", *LoopContext)`) to make it accessible inside callbacks without global vars.

### Callback Layout

- `before_<event>`: evaluate guards (e.g., budget checks before `budget_ok`).
- `enter_process_tools`: spawn concurrent tool execution, store results into metadata.
- `enter_handle_completion`: call `responses.HandleNoToolCalls` and set metadata for output + `cont` flag.
- `enter_update_budgets`: reuse logic from `tool_executor.UpdateBudgets`, but restructure into pure functions returning `budgetOutcome`.
- `enter_finalize`: trigger `memory.StoreAsync` and logging.
- `after_<event>`: push iteration transcripts (messages + results) onto `llmReq.Messages` just once per iteration.

### Guard Strategies

- Budget guard: adapt existing counter logic to functions (`func guardBudget(ctx *LoopContext) (bool, error)`), invoked from `before_budget_ok` and `before_budget_exceeded` callbacks.
- No-progress guard: move `buildIterationFingerprint` usage into `before_budget_ok` to detect stagnation before returning to `await_llm`.
- Structured retry guard: maintain in `LoopContext` and evaluated before firing `completion_retry`.

### Logging & Metrics

- Centralize instrumentation inside `after_<event>` hooks so each transition logs iteration, cumulative budgets, and tool outcomes via `logger.FromContext(ctx)`.
- Add optional `before_event` callback to emit metrics/trace spans (replacing dispersed logging in `loop.go` and `tool_executor.go`).

### Handling Clarification Gap

- The FSM still makes room for a future `request_clarification` state by reserving events but currently leaving them unused. Because we operate as an orchestration backend, transitions skip the question state until we expose an interaction channel.

## Implementation Tasks

1. **Introduce FSM scaffolding**
   - Create `engine/llm/orchestrator/state_machine.go` with FSM builder (`newLoopFSM`) returning `*fsm.FSM` + `LoopContext`.
   - Define constants for states/events to avoid typos (consider `const (StateAwaitLLM = "await_llm" ...)`).
   - Add helper to fetch metadata safely (`func loopCtx(e *fsm.Event) *LoopContext`).
2. **Refactor loop execution**
   - Replace `conversationLoop.Run` body with FSM-driven loop: initialize FSM, trigger `start_loop`, then iterate on `fsm.Event` results until terminal state.
   - Move LLM invocation into `enter_await_llm` callback that sets metadata for `llm_response` event.
   - Convert existing branches to FSM events (`response_no_tool`, `response_with_tools`) emitted based on response shape.
3. **Port tool execution logic**
   - Move concurrency code from `tool_executor.Execute` into `enter_process_tools`, preserving `errgroup` usage.
   - Expose `ToolExecutor.Execute` as smaller unit returning results without directly mutating budgets (the FSM handles counters).
4. **Centralize budget/progress guards**
   - Extract logic from `tool_executor.UpdateBudgets` and `loop_state.detectNoProgress` into guard functions invoked from `before_budget_ok`.
   - Keep `LoopContext` as the source of truth for counters to simplify testing.
5. **Rewire response handler**
   - Wrap `responses.HandleNoToolCalls` inside `enter_handle_completion`. Translate returned `cont`/`output` into events `completion_retry` or `completion_success`.
6. **Finalize output & memory sync**
   - On `enter_finalize`, call `memory.StoreAsync` (existing behavior) and return the `core.Output` stored in metadata to `conversationLoop.Run`.
   - Ensure `orchestrator.Execute` reads final metadata instead of return values from `Run`.
7. **Error propagation**
   - Implement `failure` event to handle errors from any callback. Maintain compatibility with existing error types (e.g., `core.Error`, `ErrNoProgress`).
8. **Config wiring**
   - Replace struct field `loop *conversationLoop` with `stateMachineFactory` or embed builder function so tests can inject mocks.
9. **Documentation & diagrams**
   - Update `docs/` with a state diagram (e.g., Mermaid) reflecting the new FSM.

## Testing Strategy

- Update unit tests in `engine/llm/orchestrator/` to target FSM transitions:
  - New tests for state guards (budget overflow, success loops).
  - Ensure existing tests like `orchestrator_execute_test.go` still cover success/failure scenarios; adapt mocks for new builder interface.
  - Add golden test verifying metadata is stored correctly for `finalize` event.
- Introduce integration-style test verifying that repeated tool successes trigger the guard-driven warning path without infinite loops.
- Keep concurrency tests for tool execution to ensure semantics unchanged.

## Observability & Deployment Notes

- Add iteration/state labels to structured logs so pipelines can trace transitions.
- Hook metrics (if available) inside `after_event` for durations per state.
- The FSM is now the sole runtime path; there are no feature flags or legacy loop fallbacks to maintain.

## Risks & Mitigations

- **Increased complexity**: Additional abstraction might obscure simple flows; mitigate with diagram + documentation.
- **Guard regressions**: Misplaced budget guard could change behavior; mitigate with focused tests covering thresholds.
- **FSM library dependency**: Introduces external package; vendor version via `go.mod` and pin commit to avoid drift.

## Follow-ups & Open Questions

- Evaluate adding asynchronous transitions (library supports deferring `Transition()`), which could enable streaming tool execution without blocking `Event` calls.
- Consider layering clarification state once product requirements allow user input loops.
- Explore instrumentation to export FSM state metrics for runtime dashboards.

## Sample FSM Builder (Sketch)

```go
package orchestrator

import (
    "context"

    "github.com/looplab/fsm"
)

func newLoopFSM(ctx context.Context, deps loopDeps, loopCtx *LoopContext) *fsm.FSM {
    return fsm.NewFSM(
        StateInit,
        fsm.Events{
            {Name: EventStartLoop, Src: []string{StateInit}, Dst: StateAwaitLLM},
            {Name: EventLLMResponse, Src: []string{StateAwaitLLM}, Dst: StateEvaluateResponse},
            {Name: EventResponseNoTool, Src: []string{StateEvaluateResponse}, Dst: StateHandleCompletion},
            {Name: EventResponseWithTools, Src: []string{StateEvaluateResponse}, Dst: StateProcessTools},
            {Name: EventToolsExecuted, Src: []string{StateProcessTools}, Dst: StateUpdateBudgets},
            {Name: EventBudgetOk, Src: []string{StateUpdateBudgets}, Dst: StateAwaitLLM},
            {Name: EventBudgetExceeded, Src: []string{StateUpdateBudgets}, Dst: StateTerminateError},
            {Name: EventCompletionRetry, Src: []string{StateHandleCompletion}, Dst: StateAwaitLLM},
            {Name: EventCompletionSuccess, Src: []string{StateHandleCompletion}, Dst: StateFinalize},
            {Name: EventFailure, Src: []string{StateAwaitLLM, StateProcessTools, StateHandleCompletion, StateUpdateBudgets}, Dst: StateTerminateError},
        },
        fsm.Callbacks{
            "enter_" + StateAwaitLLM: func(ctx context.Context, e *fsm.Event) { deps.invokeLLM(ctx, loopCtx) },
            "enter_" + StateProcessTools: func(ctx context.Context, e *fsm.Event) { deps.executeTools(ctx, loopCtx) },
            "enter_" + StateHandleCompletion: func(ctx context.Context, e *fsm.Event) { deps.handleCompletion(ctx, loopCtx) },
            "enter_" + StateUpdateBudgets: func(ctx context.Context, e *fsm.Event) { deps.evaluateBudgets(ctx, loopCtx) },
            "enter_" + StateFinalize: func(ctx context.Context, e *fsm.Event) { deps.finalize(ctx, loopCtx) },
        },
    )
}
```

## Zen MCP Refactor Findings

- Initial refactor analysis confirmed that the orchestrator loop encodes transitions implicitly inside `conversationLoop.Run` and distributes guard logic across `tool_executor` and `loop_state`, reinforcing the need for an explicit FSM.
- Additional tool guidance highlighted the importance of documenting refactor opportunities with precise locations; we reviewed `loop.go`, `tool_executor.go`, `loop_state.go`, and `progress.go` to capture transition and guard refactors accordingly. (Subsequent attempts to complete the automated step failed due to repeated validation prompts; manual notes are captured above.)

## Next Steps

1. Align with product/PRD owners to validate the state chart and ensure clarification state remains out-of-scope for now.
2. Create a spike branch implementing the FSM scaffold and migrate one pathway (e.g., tool execution) to vet testing approach before full rewrite.
3. Run `make lint` and `make test` to validate the branch; augment test suites as described.
4. Document the state machine in developer docs and communicate rollout plan to teammates.
