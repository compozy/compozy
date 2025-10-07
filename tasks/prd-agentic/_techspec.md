# Agentic Built-in Tool Tech Spec

## Overview
Compozy workflows today require task authors to pin a single `task.Config.Agent` or `task.Config.Tool` for each basic task. Building multi-agent behaviors demands explicit YAML wiring or bespoke workflow types. The ask is to expose a **native builtin tool** that agents can invoke from within a prompt to orchestrate one or more other agents (and their actions) sequentially or in parallel. This enables higher-order, agentic behaviors without modifying workflow configuration, and leverages the recently delivered synchronous execution APIs.

## Goals
- Provide a first-class builtin (`cp__agent_orchestrate`) that agents can call via function/tool calling to execute one or more agents/actions from natural language instructions.
- Support sequential and parallel execution blocks with explicit data passing between steps.
- Reuse existing synchronous execution plumbing (`tkrouter.DirectExecutor`, `agent/router`) instead of duplicating orchestration logic.
- Surface rich metadata (exec IDs, outputs, errors) for every sub-execution so downstream tasks can reason about results.
- Enforce safety limits (recursion depth, concurrency, timeouts) and preserve observability/telemetry parity with other cp__ tools.

## Non-Goals
- Asynchronous orchestration / background execution (HTTP polling endpoints already cover async; builtin remains sync).
- Auto-registration of new agents or actions.
- Dynamic mutation of workflow state beyond creating child executions in `task.Repository`.
- General-purpose workflow compilation (scope is limited to orchestrating agent/action calls).

## Current State
### Synchronous agent execution
- HTTP POST `/agents/{agent_id}/executions` (`engine/agent/router/exec.go`) builds a transient `task.Config` around the target agent/action.
- Execution uses `tkrouter.ResolveDirectExecutor` to obtain a `DirectExecutor` tied to the current `appstate.State` and `task.Repository`.
- `DirectExecutor.ExecuteSync` (`engine/task/router/direct_executor.go`) normalizes config, spawns runtime, and persists `task.State` entries.

### Builtin tool plumbing
- Builtins live under `engine/tool/builtin` and register via `engine/tool/native.Definitions()` into `llm.Service` (`engine/llm/service.go`).
- Tool handlers receive `context.Context` seeded with `logger`, `config.Manager`, request IDs, etc., but no direct accessor to `appstate.State` or repositories.

### Tool execution context gaps
- Existing agent router helpers (`loadAgentConfig`, `prepareAgentExecution`, etc.) are package-local to HTTP layer.
- No reusable service exports synchronous agent execution for internal callers.
- Builtin handlers lack a sanctioned way to reach `resources.ResourceStore`, `task.Repository`, or `DirectExecutor`.

## Proposed Solution
### High-level architecture
1. **Plan compilation**: Interpret the builtin input (structured JSON + optional natural language prompt) into an `AgentExecutionPlan` composed of sequential steps and parallel groups.
2. **Execution engine**: Use a shared `agentexec.Runner` service to execute each plan node via `DirectExecutor`. Maintain execution context (variable bindings, prior outputs) across steps and combine results for parallel blocks.
3. **Builtin handler**: Wrap the compiler + engine behind a cp__ tool handler that handles telemetry, validation, safety limits, and response formatting.

### Builtin definition
- ID: `cp__agent_orchestrate`
- Description: "Compile and execute multi-agent plans expressed inline from the calling prompt."
- Input schema outline:
  ```json
  {
    "type": "object",
    "properties": {
      "prompt": {"type": "string", "description": "Natural language instructions"},
      "plan": {"$ref": "#/definitions/plan"},
      "bindings": {"type": "object", "additionalProperties": {}},
      "timeout_ms": {"type": "integer", "minimum": 1},
      "max_parallel": {"type": "integer", "minimum": 1}
    },
    "oneOf": [{"required": ["plan"]}, {"required": ["prompt"]}]
  }
  ```
- Output schema outline:
  ```json
  {
    "type": "object",
    "required": ["success", "steps"],
    "properties": {
      "success": {"type": "boolean"},
      "steps": {
        "type": "array",
        "items": {
          "type": "object",
          "required": ["id", "type", "status"],
          "properties": {
            "id": {"type": "string"},
            "type": {"enum": ["agent", "parallel"]},
            "status": {"enum": ["success", "failed", "partial", "skipped"]},
            "exec_id": {"type": "string"},
            "outputs": {},
            "error": {},
            "children": {"type": "array"}
          }
        }
      }
    }
  }
  ```

### Plan representation
Define new structs under `engine/tool/builtin/orchestrate/plan.go`:
- `Plan` (root) with `[]Step`.
- `Step` union (agent vs. parallel):
  - `AgentStep` containing `AgentID`, optional `ActionID`, optional `Prompt`, optional `With` (templated input map), and `ResultKey` to store outputs into the plan context.
  - `ParallelStep` containing `[]AgentStep`, `MaxConcurrency`, and `MergeStrategy` (e.g., `collect`, `first_success`).
- Support referencing previous outputs via handlebars-like expressions in `With` (reuse `tplengine` to render against current bindings).

### Plan compiler
Create `planner.Compiler` (new package under `engine/tool/builtin/orchestrate/planner`).
- **Structured path**: Validate supplied `plan` JSON against schema -> map into plan structs.
- **Prompt path**: When only `prompt` is provided, call an internal planning routine:
  - Construct a synthetic agent config with deterministic instructions that request JSON plan output.
  - Invoke `agentexec.Runner` with `toolChoice` disabled and recursion depth guard (see Safety) to prevent calling `cp__agent_orchestrate` recursively.
  - Parse returned JSON into plan structs; reject non-conforming responses.
- Compiler merges `bindings` into initial context and enforces max step counts to prevent runaway plans.

### Shared agent runner service
Introduce `engine/agent/exec` package (or `engine/agent/service/runner`):
- `type Runner struct { state *appstate.State; taskRepo task.Repository; directFactory tkrouter.DirectExecutorFactory; resourceStore resources.ResourceStore }`
- `func NewRunner(state *appstate.State, repo task.Repository, store resources.ResourceStore) (*Runner, error)`
- `func (r *Runner) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResult, error)` wraps the logic currently spread across `agent/router/exec.go` (`loadAgentConfig`, `buildTaskConfig`, `prepareAgentExecution`, `runAgentExecution`).
- Expose options for timeouts, idempotency key, metadata to support reuse by HTTP layer and builtin.
- Update router code to call the new service (ensures single orchestration path).

### Dependency plumbing for builtin
- Extend `engine/task/uc/exec_task.go` to capture execution dependencies **before** calling `llmService.GenerateContent`:
  - Retrieve `appstate.State`, `task.Repository`, `resources.ResourceStore` from `DirectExecutor` (already available via struct fields) and attach to context via a new package `engine/tool/context` using typed keys (e.g., `toolcontext.WithAppState(ctx, state)`).
  - Ensure child goroutines inherit context with these values.
- Builtin handler fetches dependencies via `toolcontext.From(ctx)`; if unavailable, return `builtin.Internal` error clarifying unsupported environment.

### Execution engine
Implement `executor.Engine` under `engine/tool/builtin/orchestrate/executor`.
- `Run(ctx, plan, runner, safetyLimits) -> ([]StepResult, error)`.
- Sequential steps executed in order; each agent call obtains a per-step timeout derived from `min(planStepTimeout, remainingParentDeadline)`.
- Parallel steps use `errgroup.Group` with concurrency cap `min(planStep.MaxConcurrency, safetyLimits.MaxParallel, len(children))`.
- Each agent execution result captured as `StepResult` with `Status`, `ExecID`, `Output`, `Error`.
- When parallel block completes, merge outputs per `MergeStrategy` (default `collect_all` creates map keyed by `ResultKey`).
- Step results stored back into shared binding map under `result.<ResultKey>` for downstream template resolution.

### State machine orchestration
- Reuse `github.com/looplab/fsm` (same dependency already vetted in `engine/llm/orchestrator`) to drive executor control flow with explicit states: `plan_init`, `planner_active`, `plan_validated`, `dispatching_step`, `awaiting_results`, `merging_results`, `completed`, `failed`.
- Define events (`start_plan`, `planner_finished`, `validation_failed`, `step_succeeded`, `step_failed`, `parallel_complete`, `timeout`, `panic`) and keep them snake-case to match existing instrumentation conventions.
- House FSM wiring in `engine/tool/builtin/orchestrate/fsm.go`; expose constructors that accept `context.Context`, plan/executor dependencies, and transition observers mirroring `transitionObserver` from `engine/llm/orchestrator/state_machine.go`.
- Record `before_event`, `enter_state`, and `after_event` callbacks using `logger.FromContext(ctx)` plus metrics hooks so every transition produces traceable telemetry.
- Ensure executor tests assert both business outcomes and FSM transition tables (e.g., planner failure jumps straight to `failed`, successful parallel branch hits `merging_results` before `completed`).

### Safety limits & recursion guards
- Track recursion depth using context key (`toolcontext.IncrementAgentOrchestrationDepth`). Deny execution once `depth > config.Runtime.AgentOrchestrator.MaxDepth` (default 3).
- Enforce `MaxSteps` (e.g., 12) and `MaxParallel` (e.g., 4) with override from builtin input (bounded by config caps).
- Propagate cancellations: create per-step context via `context.WithTimeout(ctx, stepTimeout)`; ensure `DirectExecutor.ExecuteSync` receives this context.
- Prevent planner self-invocation by marking context with `toolcontext.DisablePlannerTools` before calling the synthetic planner agent.

### Telemetry & observability
- Use `builtin.RecordInvocation` with tool ID, status, total duration, cumulative response size (size of serialized step outputs truncated to cap).
- Emit structured logs per step via `logger.FromContext(ctx)` including agent ID, action ID, exec ID, latency, and status.
- Optionally push OpenTelemetry events via `toolLatencySeconds` (reuse existing histogram by logging per-step as part of builtin logging).

### Response payload
Return object with:
- `success`: true iff all plan steps succeeded.
- `steps`: array reflecting original plan order; parallel steps include child array.
- `bindings`: final map of named outputs (filtered to JSON-safe primitives).
- `errors`: optional summary array when failures occur.

## Implementation Plan
1. **Agent runner extraction**
   - Create `engine/agent/exec/runner.go`; migrate reusable helpers from `agent/router/exec.go`.
   - Update router to depend on new runner (ensures parity & reduces duplication).
2. **Tool context bridge**
   - Add `engine/tool/context/context.go` with setters/getters for app state, repos, recursion depth flags.
   - Modify `DirectExecutor.ExecuteSync` (or higher-level call in `executeOnce`) to inject these values before invoking `ExecuteTask`.
3. **Plan & executor packages**
   - Implement plan structs + validation under `engine/tool/builtin/orchestrate/plan.go`.
   - Implement compiler and executor subpackages as described.
4. **Builtin handler**
   - New package `engine/tool/builtin/orchestrate/handler.go` implementing `builtin.BuiltinDefinition`.
   - Register definition in `engine/tool/native/catalog.go` (append to definitions slice).
   - Wire telemetry + error handling consistent with other builtins.
5. **Documentation & samples**
   - Update `AGENTS.md` and relevant configuration guides to describe builtin usage.
   - Provide YAML snippet demonstrating prompt-only invocation leading to multi-agent coordination.
6. **Configuration hooks**
   - Extend `config.Runtime.NativeTools` with optional `AgentOrchestrator` limits (max depth, max parallel, max steps, planner model override).
7. **Testing**
   - Unit tests for planner parsing (prompt & structured), executor sequencing/parallel branches, recursion guard, and error cases.
   - Integration test driving the builtin via `llm/service` dynamic mock to guarantee compatibility with agent tool calling pipeline.

## Testing Strategy
- **Unit**: plan validation, prompt compilation (mocked runner), executor concurrency, template binding resolution, telemetry error branches.
- **Integration**: end-to-end run using in-memory resource store and `DynamicMockLLM` to simulate orchestrated agent responses.
- **Regression**: HTTP sync endpoint tests rerun to confirm runner refactor preserves behavior.
- **Load/Safety**: targeted benchmark ensuring parallel fan-out respects configured concurrency and gracefully handles cancellations.

## Observability & Monitoring
- Add dedicated logger keys (`tool=cp__agent_orchestrate`, `step_id`, `agent_id`, `exec_id`).
- Emit gauge/counter for recursion depth rejections via metrics.
- Ensure sub-execution states remain queryable via `/executions/agents/{exec_id}`.

## Security & Limits
- Enforce same sandbox rules as HTTP agent execution (no additional file/network privileges).
- Respect project-level ACLs by reusing resource store scoped through `appstate.State`.
- Guard against untrusted prompt injection by requiring planner to return schema-conformant JSON; reject free-form responses.

## Rollout Plan
1. Land runner refactor + router migration.
2. Behind feature flag (config `runtime.native_tools.agent_orchestrator_enabled`) register builtin.
3. Shadow test via staged environment with synthetic workflows.
4. Enable by default once telemetry confirms stability.

## Open Questions / Follow-ups
- Should planner support custom instructions per call (e.g., `planner_agent_id`)? Default plan is to use a fixed system prompt; configurable override could be future work.
- How should long-running agent chains report partial progress back to caller? Current design returns only final aggregated result; streaming may be iteration 2.
- Do we need per-step retry policies? Not in scope for first release; consider after initial adoption feedback.

## Appendix: Key References
- `engine/agent/router/exec.go` — current sync execution workflow.
- `engine/task/router/direct_executor.go` — task execution internals.
- `engine/llm/service.go` — builtin registration + tool executor lifecycle.
- `engine/tool/builtin` — existing cp__ tool implementations for metrics/error patterns.
