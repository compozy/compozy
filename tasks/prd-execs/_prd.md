# Product Requirements Document (PRD) Template

## Overview

We will add first-class API endpoints that let clients execute work without orchestration boilerplate:

- Execute a workflow synchronously in a single request: `POST /api/v0/workflows/{workflow_id}/executions/sync`.
- Execute an agent directly (sync and async): `POST /api/v0/agents/{agent_id}/executions` and `POST /api/v0/agents/{agent_id}/executions/async`.
- Execute a task directly (sync and async): `POST /api/v0/tasks/{task_id}/executions` and `POST /api/v0/tasks/{task_id}/executions/async`.

This reduces friction versus the current two-step pattern (trigger then poll), enables “just run this agent/task” developer flows, and improves DX for quick, blocking runs during development and automation.

## Goals

- Reduce client round-trips for common runs: enable single-call workflow completion for short-lived workloads.
- Provide direct agent and task execution surfaces that don’t require pre-defining a workflow.
- Establish consistent execution semantics across workflows, agents, and tasks (timeout, idempotency, errors).
- Telemetry targets (initial):
  - p95 latency for synchronous runs ≤ 5s for trivial tasks (no network/tool I/O).
  - 0.1% or lower HTTP 5xx rate for these endpoints over 7-day rolling windows.
  - ≥ 30% of new programmatic executions adopt the new endpoints within 30 days of GA.

## User Stories

- As an app developer, I can run a workflow synchronously to get the final output in one call while prototyping.
- As an automation service, I can run an agent with a prompt (or action) and receive the answer in the HTTP response, or a handle to poll later.
- As a backend job, I can execute a single task (agent/tool/direct LLM task) with inputs and receive output without creating a workflow.
- As a platform admin, I can enforce timeouts, idempotency, and rate limits so these endpoints are safe to operate at scale.

## Core Features

Feature 1 — Synchronous Workflow Execution

- What: `POST /workflows/{workflow_id}/executions/sync` blocks until the workflow completes or timeout.
- Why: Eliminates the two-call pattern for short runs; improves DX.
- High-level behavior:
  - Request body mirrors current execute payload (`input`, optional `task_id`).
  - Optional `timeout` (seconds) controls max server wait; default 60s, max 300s.
  - Idempotency via `Idempotency-Key` header (≤ 50 chars, 24h TTL).
  - Response 200 on success with final workflow state/output; 408 on server-side timeout with partial status if available; 202 not used for sync.
  - Errors follow `router.Response` problem envelope.
- Functional requirements (R1–R7):
  1. R1: Accept `POST /workflows/{workflow_id}/executions/sync` with JSON body `{ input: object, task_id?: string, timeout?: number }`.
  2. R2: Validate `workflow_id` exists; 404 when not found.
  3. R3: Enforce `timeout` default 60s, maximum 300s; 400 when invalid.
  4. R4: Honor `Idempotency-Key` to deduplicate requests (same route+body hash) within 24h; return prior outcome.
  5. R5: On completion, return 200 with `{ data: { workflow: State, output, exec_id } }` plus standard headers.
  6. R6: On server-side timeout, return 408 with `{ error, data?: { exec_id, state } }` and leave execution running.
  7. R7: Log/trace with exec correlation ID; emit metrics for latency, outcome, and timeouts.

Feature 2 — Direct Agent Execution (Sync and Async)

- What: Execute an agent without a workflow.
- Endpoints:
  - Sync: `POST /agents/{agent_id}/executions`
  - Async: `POST /agents/{agent_id}/executions/async`
- Why: Allow quick agent calls (prompt or action) without defining a workflow.
- High-level behavior:
  - Request body supports agent inputs: `{ action?: string, prompt?: string, with?: object, timeout?: number }`.
  - If neither `action` nor `prompt` is provided → 400.
  - Sync returns 200 with agent `output` (and optional `usage/meta`).
  - Async returns 202 with `{ exec_id, exec_url }` and `Location: /api/v0/executions/agents/{exec_id}`.
  - Idempotency via `Idempotency-Key`.
- Functional requirements (R8–R16): 8) R8: Accept `POST /agents/{agent_id}/executions` with above schema; validate agent exists or 404. 9) R9: Enforce timeout defaults (60s default, 300s max) for sync; async ignores `timeout` (queued execution). 10) R10: Return 200 with `{ data: { output, usage?: object, exec_id?: string } }` for sync. 11) R11: Async variant returns 202 + `Location` header and `{ data: { exec_id, exec_url } }`. 12) R12: Provide `GET /executions/agents/{exec_id}` to fetch status/result for async. 13) R13: Support `Idempotency-Key` (route+body hash) with 24h TTL. 14) R14: Enforce authN/Z: callers must have permission to run the agent. 15) R15: Standard error semantics with consistent problem codes. 16) R16: Emit metrics (latency, error rate, timeouts) and traces.

Feature 3 — Direct Task Execution (Sync and Async)

- What: Execute a task without a workflow (agent/tool/direct LLM task).
- Endpoints:
  - Sync: `POST /tasks/{task_id}/executions`
  - Async: `POST /tasks/{task_id}/executions/async`
- High-level behavior:
  - Request body `{ with?: object, timeout?: number }`; for agent-backed tasks the agent/action rules apply via task config.
  - Sync returns 200 with task `output`.
  - Async returns 202 + `Location: /api/v0/executions/tasks/{exec_id}` with `{ exec_id, exec_url }`.
  - Idempotency via `Idempotency-Key`.
- Functional requirements (R17–R24): 17) R17: Accept schema and validate the `task_id` exists; 404 when not found. 18) R18: Sync timeout defaults (60s default, 300s max); async ignores `timeout`. 19) R19: Return 200 with `{ data: { output, exec_id?: string } }` for sync. 20) R20: Async returns 202 + `Location` and `{ data: { exec_id, exec_url } }`. 21) R21: Provide `GET /executions/tasks/{exec_id}` to fetch status/result for async. 22) R22: Idempotency as above; deduplicate on route+body. 23) R23: AuthN/Z enforced for task execution. 24) R24: Emit metrics and traces.

Developer Experience (shared)

- Responses use standard `router.Response` envelope.
- Document comprehensive examples (cURL) for all endpoints and outcomes (200/202/400/404/408/409/422/500).
- Return `Location` header for all async 202 responses pointing to canonical status URL.

## User Experience

Examples (trimmed):

- Sync workflow
  - `POST /api/v0/workflows/greeter/executions/sync`
  - Body: `{ "input": {"name":"World"}, "timeout": 30 }`
  - 200: `{ "data": { "workflow": { ...final_state }, "output": {...}, "exec_id": "XYZ" } }`

- Async agent
  - `POST /api/v0/agents/code-reviewer/executions/async`
  - Body: `{ "prompt": "Review this code", "with": {"code":"..."} }`
  - 202 + `Location: /api/v0/executions/agents/ABC`
  - Body: `{ "data": { "exec_id": "ABC", "exec_url": "/api/v0/executions/agents/ABC" } }`

## High-Level Technical Constraints

- Authentication/Authorization: only authorized principals can execute workflow/agent/task resources (project scoping applies).
- Timeouts: sync endpoints must enforce server-side timeout (default 60s, max 300s), cancel underlying work where supported, and return 408.
- Idempotency: `Idempotency-Key` header required for production clients; dedupe scope = (method + route + normalized body hash); TTL 24h.
- Rate limiting: per-project + per-IP ceilings to prevent abuse; document default ceilings.
- Payload caps: request bodies ≤ configured max (document default, e.g., 1 MiB) with 413 when exceeded.
- Observability: metrics for counts/latency/timeouts/error classes; tracing with `exec_id` correlation.
- Compatibility: does not remove or change existing routes; adds new ones.

## Non-Goals (Out of Scope)

- No SSE/WebSocket/Server-Sent streaming in MVP (consider in Phase 2/3).
- No UI changes; API-only.
- No guarantees of exactly-once side effects at the agent/tool level (best-effort idempotency at API layer only).
- No custom schedulers or priority queues introduced in MVP.

## Phased Rollout Plan

- MVP (v1):
  - Add `POST /workflows/{workflow_id}/executions/sync`.
  - Add direct agent/task execution sync+async endpoints.
  - Add status endpoints: `GET /executions/agents/{exec_id}`, `GET /executions/tasks/{exec_id}`.
  - Docs and OpenAPI updates.

- Phase 2:
  - Webhook callback support for async completions.
  - Execution logs/percent progress in status payloads.
  - SDK helpers (Go/JS) for sync/async flows.

- Phase 3:
  - Streaming outputs (SSE) for long-running synchronous runs.
  - Advanced controls (cancel/retry) on exec resources.

## Success Metrics

- Adoption: share of new executions using direct endpoints ≥ 30% within 30 days of GA.
- Reliability: ≤ 0.1% 5xx for these endpoints.
- Latency: p95 ≤ 5s for trivial sync runs (no external I/O); measure and report weekly.
- Support: reduction in “how to run agent/task quickly” tickets by ≥ 50% quarter-over-quarter.

## Risks and Mitigations

- Long-running requests can tie up server resources → enforce strict timeouts and recommend async or webhooks.
- Duplicate submissions and side effects → require/use `Idempotency-Key`; document caveats for side-effects.
- Abuse/DoS via sync runs → rate limits and authZ checks; payload caps.
- Ambiguous agent inputs → strict validation (must have `action` or `prompt`).

## Open Questions

- Confirm default and max timeout values (proposal: default 60s, max 300s).
- Confirm `Idempotency-Key` TTL (proposal: 24h) and conflict semantics for in-flight vs finished duplicates.
- Confirm standard problem codes to use for validation (`422` vs `400`) across endpoints.
- Confirm required auth scopes for direct task/agent executions.
- Confirm maximum payload size and content-type constraints.

## Appendix

- Design alternative (future consideration): collapse sync/async to a single endpoint using `Prefer: respond-async` or `?async=true` to reduce surface area; return 200 for sync and 202 + `Location` for async.
- Canonical status resources proposed:
  - `GET /executions/workflows/{exec_id}` (already exists)
  - `GET /executions/agents/{exec_id}`
  - `GET /executions/tasks/{exec_id}`
- Response envelope examples will follow `router.Response` with `data` for success and `error` for problems; include examples in API docs.
