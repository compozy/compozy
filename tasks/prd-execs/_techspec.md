# PRD-Execs Technical Specification

## Executive Summary

This spec implements first-class execution endpoints to run workflows synchronously and to execute agents and tasks directly (sync and async). It builds on existing router, repository, and worker patterns to provide:

- Sync workflow execution: `POST /api/v0/workflows/{workflow_id}/executions/sync` with server-side wait bounded by a caller-specified timeout (default 60s, max 300s).
- Direct agent execution: `POST /api/v0/agents/{agent_id}/executions` (sync) and `/executions/async` (202 + Location).
- Direct task execution: `POST /api/v0/tasks/{task_id}/executions` (sync) and `/executions/async` (202 + Location).
- Status endpoints for async executions: `GET /api/v0/executions/agents/{exec_id}` and `GET /api/v0/executions/tasks/{exec_id}`.
- Idempotency via `X-Idempotency-Key` and API-scoped Redis keys with 24h TTL.

The design reuses:

- Router patterns under `engine/*/router` and standard response envelope.
- Task execution core via `task/uc.ExecuteTask` for agent/tool/direct LLM paths.
- Workflow state repo polling for synchronous completion.
- Existing webhook idempotency service generalized to API execs.

## System Architecture

### Domain Placement

- workflow/
  - `engine/workflow/router/execute_sync.go` (new): synchronous workflow execution handler
- agent/
  - `engine/agent/router/exec.go` (new): direct agent execution (sync/async) + status
- task/
  - `engine/task/router/exec.go` (new): direct task execution (sync/async) + status
- infra/server/router/
  - Use existing helpers (`GetAgentID`, `GetTaskID`, `GetAgentExecID`, `GetTaskExecID`, response helpers)
  - `engine/infra/server/router/idempotency.go` (new): small API idempotency helper wrapping `engine/webhook` service with API namespace and body hashing
- webhook/
  - Reuse `engine/webhook` idempotency `Service` and key helpers (no new external deps)
- infra/monitoring/
  - Add counters/timers for new endpoints

### Component Overview

- Exec Router Handlers (new): Thin HTTP handlers validating input, enforcing timeouts/limits, applying idempotency, and delegating to use cases.
- Exec Use Cases (new minimal service functions colocated with routers):
  - WorkflowSync: trigger via worker, then poll workflow repo until terminal or timeout.
  - AgentExec/TaskExec: construct transient basic task config and call `uc.ExecuteTask`; persist task state before/after execution for status.
- API Idempotency Helper (new): Derive key with clear precedence and semantics, namespace under `idempotency:api:execs:` and set TTL 24h.
  - Precedence: if `X-Idempotency-Key` header is present, use it; otherwise compute a stable hash of `(method + route + normalized JSON body)`.
  - In-flight duplicates: if a second request arrives with the same key while the first is still processing, respond `409 Conflict` immediately (do not block). After the first completes, subsequent identical requests return the original outcome per dedupe policy.
- Metrics: timers for sync wait, counts by outcome (success/timeout/error), and 5xx rate.

## Implementation Design

### Core Interfaces

```go
// engine/infra/server/router/idempotency.go (new)
type APIIdempotency interface {
    // CheckAndSet ensures one execution per dedupe key (24h TTL by default).
    // Returns (true, "") when unique; returns (false, reason) on duplicate.
    CheckAndSet(ctx context.Context, c *gin.Context, namespace string, body []byte, ttl time.Duration) (bool, string, error)
}

// engine/agent/router/exec.go (new)
// minimal service used by handlers; relies on existing repos and uc.ExecuteTask
type DirectExecService interface {
    // Executes an agent task synchronously and returns final output and persisted task state ID
    ExecuteAgentSync(ctx context.Context, agentID string, req AgentExecRequest) (*core.Output, core.ID, error)
    // Starts async agent execution and returns exec ID
    ExecuteAgentAsync(ctx context.Context, agentID string, req AgentExecRequest) (core.ID, error)
    // Loads persisted task state by exec ID (for status endpoint)
    GetTaskState(ctx context.Context, execID core.ID) (*task.State, error)
}
```

### Data Models

- AgentExecRequest (request body)
  - `action?: string`
  - `prompt?: string`
  - `with?: object`
  - `timeout?: number` (sync only; default 60s; max 300s)
  - Note: At least one of `action` or `prompt` must be provided.
- AgentExecSyncResponse (200 OK)
  - `{ data: { output: core.Output, exec_id: string } }`
- AgentExecAsyncResponse (202 Accepted)
  - `{ data: { exec_id: string, exec_url: string } }` + `Location: /api/v0/executions/agents/{exec_id}`
- TaskExecRequest/Responses mirror AgentExec\* with `task_id` path param implied.
- WorkflowSyncRequest (request body)
  - `{ input: object, task_id?: string, timeout?: number }`
- WorkflowSyncResponse (200 OK)
  - `{ data: { workflow: workflow.State, output: core.Output, exec_id: string } }`

Persistence:

- Use `task.Repository.UpsertState` to persist Task state with `TaskExecID`, `Status`, `Input`, `Output`/`Error` for direct agent/task execs.
- Reuse `workflow.Repository.GetState` for workflow sync polling.

### API Endpoints

- `POST /api/v0/workflows/{workflow_id}/executions/sync`
  - Blocks until terminal or timeout (default 60s, max 300s). 200 on success, 408 on server-side timeout (execution may continue), standard problem on errors.
- `POST /api/v0/agents/{agent_id}/executions` (sync)
  - Requires `action` or `prompt`; 200 with `{ output, exec_id }` on success.
- `POST /api/v0/agents/{agent_id}/executions/async`
  - 202 with `{ exec_id, exec_url }` and `Location` header.
- `GET /api/v0/executions/agents/{exec_id}`
  - Always `200 OK`. Returns persisted `task.State` with `status` field (`PENDING|RUNNING|COMPLETED|FAILED`). Clients should poll and inspect `status` to detect completion.
- `POST /api/v0/tasks/{task_id}/executions` (sync)
  - Executes configured task directly; 200 with `{ output, exec_id }`.
- `POST /api/v0/tasks/{task_id}/executions/async`
  - 202 with `{ exec_id, exec_url }` and `Location` header.
- `GET /api/v0/executions/tasks/{exec_id}`
  - Always `200 OK`. Returns persisted `task.State` with `status` field; clients poll and inspect `status`.

Examples (concise):

- Agent sync request:
  - POST `/api/v0/agents/code-reviewer/executions`
  - Body:
    ```json
    { "prompt": "Summarize PR #42", "with": { "lang": "en" } }
    ```
  - 200 Body:
    ```json
    {
      "status": 200,
      "data": { "output": { "text": "..." }, "exec_id": "AGT123" }
    }
    ```
- Agent async request:
  - POST `/api/v0/agents/code-reviewer/executions/async`
  - 202 Body:
    ```json
    {
      "status": 202,
      "data": {
        "exec_id": "AGT123",
        "exec_url": "/api/v0/executions/agents/AGT123"
      }
    }
    ```
- Workflow sync request:
  - POST `/api/v0/workflows/data-processing/executions/sync`
  - Body:
    ```json
    { "input": { "file": "s3://..." }, "timeout": 60 }
    ```
  - 200 Body:
    ```json
    {
      "status": 200,
      "data": {
        "workflow": { "workflow_id": "data-processing", "status": "COMPLETED" },
        "output": { "text": "..." },
        "exec_id": "WF123"
      }
    }
    ```

All endpoints honor `X-Idempotency-Key` and enforce max body size (existing server defaults). Idempotency scope = (method + route + normalized body).

## Integration Points

- Redis (idempotency store) via existing `engine/webhook` service.
- Temporal worker for workflows (unchanged). Synchronous workflow waits through repo polling (no client.GetWorkflow dependency introduced).
- No new third-party libraries.

## Impact Analysis

| Affected Component                | Type of Impact             | Description & Risk Level                              | Required Action              |
| --------------------------------- | -------------------------- | ----------------------------------------------------- | ---------------------------- |
| engine/workflow/router (new file) | API Surface (Non-breaking) | Adds `/executions/sync` endpoint. Low.                | Add handler + tests.         |
| engine/agent/router (new file)    | API Surface (Add)          | Adds direct exec + status endpoints. Low/Medium.      | Add handlers + DTOs + tests. |
| engine/task/router (new file)     | API Surface (Add)          | Adds direct exec + status endpoints. Low/Medium.      | Add handlers + DTOs + tests. |
| engine/webhook (reuse)            | Behavior (Reused)          | Reuse idempotency service; add API namespace wrapper. | Add small helper adapter.    |
| infra/monitoring                  | Metrics (Add)              | New timers/counters for exec endpoints. Low.          | Add metrics wiring.          |
| docs/swagger.\*                   | Docs (Update)              | Document new endpoints, headers, examples.            | Update OpenAPI + docs.       |

Performance: Sync waits are bounded (≤ 300s). Poll workflow repo using exponential backoff starting at 200ms, doubling up to 5s max interval, with ±10% jitter and early-exit on terminal states. Rate limits recommended at gateway.

## Testing Approach

### Unit Tests

- Router validation and error mapping for each endpoint.
- Idempotency helper: key derivation (header vs. body hash), TTL, namespace.
- DirectExecService: agent/task sync happy paths and validation (action/prompt presence).

### Integration Tests

- Workflow sync: run a lightweight workflow and verify 200 vs 408 paths.
- Agent/task async: `POST /.../async` → 202 with Location → `GET /executions/.../{exec_id}` returns terminal state after worker completes.
- Idempotency: same `X-Idempotency-Key` returns prior outcome without duplicate execution.

## Development Sequencing

### Build Order

1. Add status endpoints: `GET /executions/agents/{exec_id}`, `GET /executions/tasks/{exec_id}` (used by async flows).
2. Add API idempotency helper (`infra/server/router/idempotency.go`) wrapping webhook service.
3. Implement direct agent/task exec (sync/async) using `uc.ExecuteTask`; persist states.
4. Implement workflow sync handler with repo polling.
5. Wire metrics; update OpenAPI/docs.
6. Tests: unit then integration; run `make lint` and `make test`.

### Technical Dependencies

- Redis available for idempotency (already required by webhooks).
- Worker ready (existing readiness checks in server).

## Monitoring & Observability

- Metrics (names illustrative):
  - `http_exec_sync_latency_seconds{kind="workflow|agent|task", outcome}` summary
  - `http_exec_timeouts_total{kind}` counter
  - `http_exec_errors_total{kind, code}` counter
- Structured logs include: `exec_id`, `workflow_id|agent_id|task_id`, `timeout`, `idempotency_key`, outcome.

## Technical Considerations

### Key Decisions

- Sync workflow uses repo polling (simple, leverages existing persistence) instead of Temporal client waits.
- Idempotency header name unified as `X-Idempotency-Key` (aligned with webhook subsystem). PRD text that used `Idempotency-Key` maps to the same behavior; header doc will specify `X-Idempotency-Key`.
- Dedup scope includes method + route + normalized body (JSON stable-marshaled + no whitespace) to satisfy PRD R4/R13/R22. Normalization details: use a stable JSON encoder (sorted keys, no whitespace), UTF-8, and canonical number formatting.
- Direct agent/task execution uses `uc.ExecuteTask` to avoid duplicated execution logic.

Error mapping (router.NewRequestError):

- 400 Bad Request: invalid path params or body (missing `action|prompt`, invalid `timeout`).
- 404 Not Found: missing workflow/agent/task definitions.
- 408 Request Timeout: sync server-side timeout for workflow/agent/task waits.
- 409 Conflict: in-flight duplicate (same idempotency key).
- 503 Service Unavailable: worker not ready.
- 500 Internal Server Error: unexpected failures.

Mapping to PRD requirements (selection):

- R1–R7 (Workflow sync): new handler validates params, enforces default/max timeout, 200 on success, 408 on server-side timeout, idempotency via header, correlation logs, metrics.
- R8–R16 (Agent exec): sync/async endpoints with schema, auth via global middleware, 202 Location for async, status `GET`, idempotency, consistent errors, metrics.
- R17–R22 (Task exec): same semantics as agent; status `GET`, idempotency.

### Known Risks

- Long synchronous waits can pin server goroutines: mitigated by hard max timeout (300s) and recommending async beyond small tasks.
- Idempotency semantics with in-flight duplicates: key is treated as in-use; second request returns previous outcome when completed.
- Exact-once side effects not guaranteed at agent/tool level; documented explicitly.

### Special Requirements

- Honor `logger.FromContext(ctx)` and `config.FromContext(ctx)` in all code paths; never use globals; inherit request context in all repo/worker calls.
- Respect `.cursor/rules/test-standards.mdc` (test names `t.Run("Should...")`, testify usage) and `.cursor/rules/no_linebreaks.mdc`.

### Standards Compliance

- Architecture: follows existing router/use-case/repo boundaries (Clean Architecture).
- Go coding: stick to small, focused handlers; error handling via `router.NewRequestError`.
- APIs: align with `api-standards.mdc` (status codes, Location header, problem details).
- Backwards compatibility: additive endpoints; no breaking changes.
