# Execution Routes Remediation Plan

## Summary
- Synchronous agent and task execution endpoints (`engine/agent/router/exec.go` and `engine/task/router/exec.go`) currently return empty or minimal outputs (e.g. `{ "status": "ok" }`) despite successful model runs.
- Asynchronous execution endpoints delegate to the in-process `directExecutor.ExecuteAsync` instead of Temporal, so async jobs never appear in Temporal UI and lose worker level orchestration and resilience.
- Idempotency scaffolding (`ensureAgentIdempotency`, `ensureTaskIdempotency`, `ensureWorkflowIdempotency`) is generating unnecessary 409 conflicts and contradicts the revised requirement to ship without API idempotency.

## Current Implementation Review

### Synchronous Agent / Task Flow
1. `executeAgentSync` / `executeTaskSync` parse the payload, load configs from the resource store, and invoke `DirectExecutor.ExecuteSync`.
2. `DirectExecutor.ExecuteSync` (in `engine/task/router/direct_executor.go`) persists a pending state, marks it running, calls `ensureRuntime`, and then invokes `uc.ExecuteTask` to execute the agent/tool/LLM.
3. `uc.ExecuteTask` (inside `engine/task/uc/exec_task.go`) dispatches to `executeAgent`, `executeTool`, or `executeDirectLLM` and is responsible for producing the final `core.Output`.
4. The HTTP response serializes `output` directly: `router.RespondOK(c, "agent executed", gin.H{"output": output, ...})`.

Symptoms observed in QA: responses show `"output": {"status": "ok"}` instead of the assistant messages / structured payload expected from the agent. That indicates the `core.Output` we persist lacks meaningful content or we are mutating it before returning.

### Asynchronous Agent / Task Flow
1. `executeAgentAsync` / `executeTaskAsync` reuse the same request parsing but call `DirectExecutor.ExecuteAsync`.
2. `DirectExecutor.ExecuteAsync` persists state, marks it running, and spawns `go d.runExecution(...)` in the API process.
3. The goroutine executes `uc.ExecuteTask` directly and writes the status back into the task repository when finished.

Because the handler never involves `state.Worker`, Temporal is unaware of these executions—there is no Temporal workflow, no durable queueing, and failures bypass worker retry semantics.

### Idempotency
- Each sync handler marshals the request body and calls `router.ResolveAPIIdempotency` → `APIIdempotency.CheckAndSet(...)` with namespace `agents`, `tasks`, or `workflows`.
- Duplicate requests return an immediate 409 (e.g. the user-provided logs show the second `POST /agents/.../executions` responded with 409 “duplicate request”).
- Routes and examples (`examples/sync/api.http`) document `X-Idempotency-Key`, but the product decision is to postpone idempotency for now.

## Identified Issues & Root Causes

1. **Agent/Task sync responses lack actual model output**
   - Likely causes: the output from `uc.ExecuteTask` is being normalised to a skeleton (e.g. a success status) before returning, or state/output gets overwritten in `DirectExecutor.updateState`. Need to trace how `core.Output` propagates and confirm no post-processing drops the assistant content.
   - No regression test currently asserts realistic agent output; unit tests only expect placeholder maps (e.g. `core.Output{"foo":"bar"}`).

2. **Async executions bypass Temporal**
   - `DirectExecutor.ExecuteAsync` runs entirely inside the HTTP process, so Temporal never sees or manages those jobs. This contradicts PRD expectations (“async executions should run in Temporal as workflows do”).
   - Lack of Temporal orchestration means no visibility in Temporal UI, no retries, and task state only lives in the DB layer.

3. **Idempotency conflicts**
   - The idempotency helper is still active and returns conflict responses. Removing it requires code changes across agents/tasks/workflow sync plus associated tests and sample files.

## Proposed Fixes

### 1. Restore Sync Output Fidelity
1. Reproduce using `examples/sync` (invoke `POST /agents/{id}/executions` with a prompt) and inspect the persisted task state (`task_repo.GetState`) to understand what `core.Output` contains.
2. Instrument `uc.ExecuteTask.executeAgent` / `DirectExecutor.runExecution` to ensure the result returned by `llm.Service.GenerateContent` is propagated without mutation.
3. If the orchestrator returns structured content (e.g. messages array), ensure we return the full map. Avoid overwriting `state.Output` with fallback values in error paths.
4. Add an integration test that stubs the LLM to return a realistic payload (e.g. `core.Output{"messages": [{"role": "assistant", "content": "..."}]}`) and assert the sync response surfaces that payload verbatim.
5. Confirm workflow sync responses still include the workflow’s output map unaffected by cleanup.

### 2. Re-implement Async Execution on Temporal
1. Design a dedicated Temporal workflow (e.g. `DirectExecutionWorkflow`) that accepts a serialized `task.Config`, metadata (`ExecMetadata`), and timeout parameters.
2. Add an activity (or reuse extracted logic) that drives `uc.ExecuteTask` inside the worker process, persists `task.State` transitions (Pending → Running → Success/Failed) and writes the output/error.
3. Extend `state.Worker` with methods such as `TriggerDirectAgentExecution` / `TriggerDirectTaskExecution`:
   - Pre-persist Pending state via repository.
   - Start the Temporal workflow with deterministic IDs (`direct-agent-{execID}` / `direct-task-{execID}`) so status endpoints can correlate.
   - Handle start failures by writing Failed state and surfacing the error.
4. Update HTTP async handlers to call the worker entrypoints instead of `DirectExecutor.ExecuteAsync`. Preserve current response shape (`202` with `exec_id` + `Location`).
5. Remove the goroutine-based direct executor path or gate it for sync-only usage.
6. Update integration tests (`test/integration/server/executions_integration_test.go`) to run against the new worker-backed path (using Temporal test environment / stubs) and assert that the task state transitions reach Success.
7. Ensure Temporal histories show these executions (manual verification during QA).

### 3. Remove Idempotency for Execution Routes
1. Delete `ensureWorkflowIdempotency`, `ensureAgentIdempotency`, and `ensureTaskIdempotency` usages in the router code and drop related helper functions if they become unused.
2. Update middleware/tests that rely on `X-Idempotency-Key` (unit tests in `engine/agent/router/exec_test.go`, `engine/task/router/exec_test.go`, integration tests).
3. Remove idempotency headers from examples (`examples/sync/api.http`) and any documentation snippets generated by the PR branch.
4. Confirm rate limiting / monitoring still works without the wrapper.

### 4. Shared Execution Service Refactor (optional but recommended)
- Extract the core execution logic (state persistence, runtime bootstrapping, call into `uc.ExecuteTask`, state updates) into a shared service that can be used both by the HTTP sync path and the Temporal activity. This reduces divergence between sync/async behaviour and ensures both flows log/emit metrics identically.

### 5. Regression Coverage & Observability
- Extend status endpoint tests to cover outputs/errors for both sync and async runs (including failure paths and timeouts).
- Ensure monitoring metrics (`router.SyncExecutionMetricsScope`, `metrics.RecordError/Timeout`) are still recorded for the new paths; add Temporal activity logging for async flows.
- Document (internally) how to trace executions in Temporal & DB for debugging.

## Implementation Checklist
- [ ] Investigate sync output issue and patch code so agent/task outputs are preserved end-to-end.
- [ ] Introduce Temporal-based direct execution workflows plus worker helpers, replacing the goroutine async executor.
- [ ] Remove idempotency checks, update tests, examples, and swagger annotations.
- [ ] Refactor shared execution logic (if needed) to avoid code duplication between sync and async paths.
- [ ] Update existing unit/integration tests and add new ones covering real-world payloads and Temporal-backed async runs.
- [ ] Execute `make lint` and `make test` before handing off.

## Testing Strategy
- Unit tests: `engine/agent/router/exec_test.go`, `engine/task/router/exec_test.go`, new tests for Temporal entrypoints.
- Integration tests: refresh `test/integration/server/executions_integration_test.go` to exercise sync + async flows against the worker (using temporal test env / stubs).
- Manual QA: run `examples/sync` requests end-to-end, verify Temporal UI reflects async jobs, confirm sync responses include assistant text.
- CI: full `make lint` + `make test` suite.

## Risks & Mitigations
- **Temporal workflow design complexity**: ensure new workflow/activity obeys project standards (context inheritance, `config.FromContext`, logging). Mitigate by reusing existing worker patterns.
- **Schema / serialization drift**: serializing `task.Config` for Temporal must remain stable; add explicit protobuf/JSON struct definitions and tests.
- **Backward compatibility**: removing idempotency may affect clients relying on dedupe. Communicate change in release notes.
- **Regression in sync latency**: if sync path is reworked to poll Temporal, ensure timeout handling matches PRD (default 60s, max 300s) and metrics continue to record outcomes.

## Open Questions / Follow-ups
- Should sync executions also run through Temporal (consistent behaviour) or remain in-process once output fidelity is fixed?
- Do we need to backfill telemetry for agent/task executions in Temporal (e.g. custom search attributes) to support observability parity with workflows?
- Confirm whether we need to keep any idempotency scaffolding for future reintroduction (e.g. feature flag) or remove it entirely.
