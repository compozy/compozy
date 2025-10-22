# Streaming Support for Executions (/stream)

This document specifies the architecture, APIs, and implementation plan to add real‑time streaming to Compozy execution endpoints for Workflows, Agents, and Tasks.

- Decision: Option A — Hybrid SSE + Temporal Queries (+ Redis for live chunks)
- JSON vs text rule: Workflows always stream structured JSON. Agents/Tasks stream structured JSON only when an output schema exists; otherwise they stream text chunks (per-line) only.
- Transport: HTTP Server‑Sent Events (SSE) with `Last-Event-ID` support and heartbeats
- Live emission for agent/task text chunks: Redis Pub/Sub (low latency). We explicitly accept ephemeral chunk loss across reconnects for token/text streaming; final state remains durable via repositories.

> Go version baseline: 1.25.2. The design and snippets assume Go 1.25+ APIs (e.g., `http.NewResponseController`).

---

## 1) Architecture Overview

- Workflow layer (Temporal):
  - Workflows register a Temporal Query handler (e.g., `getStreamState`) returning a durable snapshot of the latest stream state (events + status). The API polls this query and emits SSE events to clients. This guarantees durability and resumability for workflow progress.

- API layer (SSE):
  - New GET endpoints under `/api/v0/executions/*/stream` return an SSE stream. Each SSE frame includes an increasing `id`, `event` (type), and `data` (JSON or text), with periodic heartbeats.
  - For workflow events, the API polls Temporal queries every 500 ms by default and emits only new events since the last cursor.
  - For agent/task streams:
    - If an output schema is present, the API polls the task state repository for structured changes and emits JSON events.
    - If no schema is present, the worker publishes live text chunks (per-line) to Redis Pub/Sub (`stream:tokens:<exec_id>`). The API subscribes and forwards as `llm_chunk` SSE events.

- Client layer:
  - Use `EventSource` with auto-reconnect. `Last-Event-ID` enables resume for workflow JSON events; token streams are best-effort (ephemeral) and resume from “now”. Final outputs are always retrievable via status endpoints.

Why this split?

- Workflows: need durable, ordered JSON stream → Temporal Query + polling is simple and robust.
- Agent/Task without schemas: the primary UX need is human-readable progressive text → Pub/Sub is ideal for low-latency streaming without introducing heavy durability costs.

---

## 2) Endpoints and Behaviors

New endpoints (SSE):

- `GET /api/v0/executions/workflows/:exec_id/stream`
- `GET /api/v0/executions/agents/:exec_id/stream`
- `GET /api/v0/executions/tasks/:exec_id/stream`

Common request headers/params:

- Optional `Last-Event-ID`: resume for durable JSON events (workflows; agent/task structured updates).
- Query params:
  - `poll_ms` (default: 500; min: 250; max: 2000) — polling cadence for Temporal/repo queries.
  - `events` — comma-separated allowlist (e.g., `workflow_status,tool_call,llm_chunk`).

Response headers:

- `Content-Type: text/event-stream`
- `Cache-Control: no-cache`
- `Connection: keep-alive`
- `X-Accel-Buffering: no` (for Nginx)
- `Access-Control-Allow-Origin: *` (or project CORS policy)

Connection lifecycle:

- Heartbeat `: ping\n` every 15s
- Max connection duration: 10 minutes (configurable) with server‑side idle timeout; clients reconnect automatically.

---

## 3) Event Model

All events include monotonically increasing `id` (int), `type` (string), `ts` (RFC3339Nano), and `data` (JSON or text). SSE format example:

```
id: 42
event: workflow_status
data: {"status":"RUNNING","step":"agent_start","ts":"2025-10-22T12:45:03.123456Z"}

```

Event types (initial set):

- `workflow_start` { workflowId }
- `workflow_status` { status, step?, usage? }
- `agent_start` { agentId, action? }
- `tool_call` { toolName, args }
- `llm_chunk` { content } // Agents/Tasks without schemas → text only; for JSON cases use `structured_delta`.
- `structured_delta` { partial } // Optional future: partial JSON for structured outputs.
- `error` { message, code? }
- `complete` { result?, usage?, durationMs }

JSON vs text rule:

- Workflows → always JSON objects in `data`.
- Agents/Tasks:
  - If `task.Config.OutputSchema != nil` or `agent.ActionConfig.OutputSchema != nil` → structured JSON events.
  - Otherwise → `llm_chunk` textual lines; no JSON framing of the content.

Resume semantics:

- Workflow JSON: resumption supported via `Last-Event-ID` (API caches last event index and emits only new ones based on Temporal query snapshot).
- Agent/Task structured JSON: resumption supported by repo polling & local cursoring.
- Agent/Task text chunks (Pub/Sub): best‑effort, no backfill on reconnect; final state/status is always persisted and queryable.

---

## 4) Redis Design (Pub/Sub)

Channels:

- `stream:tokens:<exec_id>` — line/chunk updates for Agent/Task executions without output schemas.

Publisher (Worker side):

- When the LLM provider returns content (final or progressively once streaming is wired), split into human-scale lines/chunks and `PUBLISH` to the channel.
- Each message payload:
  - `{ "type": "llm_chunk", "content": "...", "ts": "..." }` (JSON for transport; API unwraps to SSE with `event: llm_chunk` and plain `data: <content>`)

Subscriber (API side):

- On `/stream` for eligible execs, subscribe to `stream:tokens:<exec_id>` and forward chunks as SSE `llm_chunk` frames.
- If Redis connection drops, reconnect and continue with new messages. No persistence/backfill (keeps design simple and fast).

Rationale for Pub/Sub vs Streams:

- We prefer low-latency, ephemeral delivery for human-facing token/text flows.
- Durability is covered by workflow/task state repositories and status endpoints. If we later need resumable text, we can add a small TTL ring buffer or switch to Redis Streams per endpoint.

---

## 5) API Implementation Sketch

Shared SSE helper (new): `engine/infra/server/router/sse.go`

- `StartSSE(w, r)` — sets headers, returns flusher/controller
- `WriteEvent(w, id, event, dataBytes)` — writes `id`, `event`, `data`, `\n\n` and flushes
- `WriteHeartbeat(w)` — writes `: ping\n\n`
- Keep each helper under 50 LOC per coding standards.

Workflow stream handler (new): `engine/workflow/router/stream.go`

- Input: `:exec_id`, optional `poll_ms`, `Last-Event-ID`
- Loop every `poll_ms`:
  - `QueryWorkflow(ctx, workflowID, runID="", "getStreamState")`
  - Decode `WorkflowStreamState{ Events: []StreamEvent, Status }`
  - Emit only events with `event.ID > lastEventID` via `WriteEvent`
  - On `Status == completed` → emit `complete` and close

Agent stream handler (new): `engine/agent/router/stream.go`

- Determine schema mode from the underlying execution context/state:
  - Structured mode: poll task repo state, diff, emit JSON events; exit on terminal
  - Text mode (no schema): subscribe `stream:tokens:<exec_id>` and forward as `llm_chunk` (per-line); exit when final state indicates terminal or client disconnects

Task stream handler (new): `engine/task/router/stream.go`

- Same structured/text branching as Agent handler.

Routing additions:

- Workflow: mount under `engine/workflow/router/register.go`
- Agent: `engine/agent/router/register.go`
- Task: `engine/task/router/register.go`
- Keep handler functions ≤ 50 LOC by factoring common logic into the SSE helper.

---

## 6) Worker Emission Points

Short‑term (Phase 1 — simulated chunking):

- If provider adapters don’t yet expose token callbacks, publish “chunked lines” from the final assistant message to `stream:tokens:<exec_id>` so the client still experiences incrementally rendered text.

Long‑term (Phase 2 — true streaming):

- Plumb provider streaming callbacks through `engine/llm/adapter → orchestrator → worker emitter`, publishing each chunk to Pub/Sub as it arrives.
- Preserve existing determinism for Temporal workflows by confining real-time emission to the worker task path (not inside workflow history).

Where to hook today:

- `engine/task/uc/exec_task.go` in `executeAgent` path, right after final content is available; split into chunks and `PUBLISH`.
- Later, when streaming is available, emit on each partial response.

---

## 7) Event Schemas

SSE frames carry `id`, `event`, `data` lines. The logical payloads below apply to `data`:

- Workflow JSON events:
  - `workflow_status`: `{ "status": "RUNNING|SUCCESS|FAILED|...", "step": "...", "usage": {...}, "ts": "..." }`
  - `tool_call`: `{ "toolName": "http", "args": { ... }, "ts": "..." }`
  - `complete`: `{ "result": { ... }, "usage": {...}, "durationMs": 12345, "ts": "..." }`

- Agent/Task structured JSON events (when schemas exist):
  - `structured_delta`: `{ "partial": { ... }, "ts": "..." }` (optional future)
  - `complete`: `{ "result": { ... }, "usage": {...}, "ts": "..." }`

- Agent/Task text events (no schema):
  - `llm_chunk`: `data: <plain text line>`

All events are prefixed with an incrementing `id` to support `Last-Event-ID` where resumable (workflows, structured cases). For text `llm_chunk` via Pub/Sub, IDs are assigned by the API process only for local ordering; missed chunks are not backfilled on reconnect.

---

## 8) Security & Limits

- AuthZ: reuse the same auth middleware used by status endpoints. Stream endpoints must enforce read permissions on the corresponding execution.
- Rate limiting: cap concurrent SSE streams per node (config), and global via gateway if present.
- Timeouts: default 10 minutes per connection; client reconnects allowed.
- Payload redaction: use existing logger redaction and avoid leaking secrets in event payloads.

---

## 9) Observability

- Metrics:
  - Active streams, bytes/sec, average duration
  - Per-endpoint error rates and reconnect counts
- Logs:
  - Connect/disconnect, reason, last id
- Traces: wrap Temporal query and Redis subscribe spans

---

## 10) Backwards Compatibility

- Purely additive: `/stream` endpoints do not change existing APIs.
- Clients that don’t consume `/stream` continue to use sync/async execute + status polling.

---

## 11) Implementation Plan (Phased)

Phase 0 — Foundations

- [ ] Add `engine/infra/server/router/sse.go` helpers (headers, write, heartbeat)
- [ ] Add shared `StreamCursor` util (tracks lastEventID safely)

Phase 1 — Workflows (durable JSON)

- [ ] Workflow SSE handler `engine/workflow/router/stream.go`
- [ ] Workflow registration in `register.go`
- [ ] Temporal workflow: add `SetQueryHandler("getStreamState")` in the workflow template(s) returning `WorkflowStreamState{ Events []StreamEvent, Status }`
- [ ] Basic client example in docs (`EventSource` consumption)

Phase 2 — Agents/Tasks (structured vs text)

- [ ] Agent SSE handler `engine/agent/router/stream.go`
- [ ] Task SSE handler `engine/task/router/stream.go`
- [ ] Branch by schema presence (`OutputSchema` / `ActionConfig.ShouldUseJSONOutput()`)
- [ ] For text mode: Redis Pub/Sub subscribe/forward; close on terminal state
- [ ] For structured mode: poll repo and emit JSON deltas

Phase 3 — Worker emission

- [ ] Phase 1 shim: publish simulated per-line chunks from final message to `stream:tokens:<exec_id>`
- [ ] Phase 2 wiring: add provider streaming callbacks and publish per partial

Phase 4 — Hardening & polish

- [ ] Heartbeats, backoff, and connection limits
- [ ] CORS/headers review; `X-Accel-Buffering: no`
- [ ] Docs and examples in `docs/` + OpenAPI annotations on endpoints

---

## 12) Files to Add/Change

New:

- `engine/infra/server/router/sse.go` (SSE helpers)
- `engine/workflow/router/stream.go` (workflow stream)
- `engine/agent/router/stream.go` (agent stream)
- `engine/task/router/stream.go` (task stream)

Modified:

- `engine/workflow/router/register.go` — mount workflow stream route
- `engine/agent/router/register.go` — mount agent stream route
- `engine/task/router/register.go` — mount task stream route
- (Worker) `engine/task/uc/exec_task.go` — Phase 1 simulated chunk publisher; later real streaming plumbing via LLM adapter

Reference types already present:

- `engine/agent/action_config.go` → `ShouldUseJSONOutput()`
- `engine/task/config.go` → `OutputSchema` presence
- `engine/task/router/direct_executor_factory.go` and `engine/task/directexec` for direct executions
- Repos:
  - Workflow: `engine/workflow/repo.go`
  - Task: `engine/infra/postgres/taskrepo.go`

---

## 13) Client Consumption Example (JS)

```ts
const es = new EventSource(`/api/v0/executions/workflows/${execId}/stream`);

es.addEventListener("workflow_status", e => {
  const data = JSON.parse(e.data);
  renderStatus(data);
});

es.addEventListener("tool_call", e => {
  const data = JSON.parse(e.data);
  showTool(data);
});

es.addEventListener("llm_chunk", e => {
  appendText(e.data); // plain text line
});

es.addEventListener("complete", () => es.close());

es.onerror = () => {
  // EventSource auto-reconnects; server supports Last-Event-ID for JSON events
};
```

---

## 14) Configuration

- API:
  - `STREAM_POLL_MS_DEFAULT=500`
  - `STREAM_MAX_CONN_MINUTES=10`
  - `STREAM_HEARTBEAT_SECONDS=15`
- Redis (reuse existing worker/redis settings): dedicated keyspace prefix `stream:*`

---

## 15) Risks & Mitigations

- Missed text chunks on reconnect (Pub/Sub): acceptable by design; final output is durable. If needed later, add a small TTL ring buffer in Redis Lists per `exec_id`.
- N+1 Temporal query load: keep polling at 500 ms by default; make interval configurable and subject to rate limits.
- Proxy buffering: set `X-Accel-Buffering: no` and `Cache-Control: no-cache`.

---

## 16) References (validated with Zen MCP)

- Temporal Go SDK — queries/signals: Context7 library docs (`/temporalio/sdk-go`)
- Temporal community guidance on using queries + external streaming (Perplexity aggregation)
- Go SSE best practices (flushing, Last-Event-ID), general resources:
  - MDN Server-Sent Events
  - Example guides on SSE in Go with flush and heartbeats

(We’ll inline concrete doc links in the code comments where relevant.)

---

## 17) Rollout Plan

1. Land SSE helpers and workflow stream (Phase 1). Ship behind minor version.
2. Add agent/task streams with structured/text branching and Pub/Sub (Phase 2).
3. Add simulated chunking (Phase 3a) and later real provider streaming (Phase 3b).
4. Add docs + examples; socialize to SDK/CLI consumers.

Success criteria:

- Live dashboard shows progressive updates for workflows (JSON) and agent/task text streams for prompt-only cases.
- Reconnects are smooth for workflow JSON (Last-Event-ID works). Final outputs are always visible through status endpoints.

---

## 18) Open Items to Confirm (resolved)

- Approach: A (Hybrid SSE + Temporal Queries + Redis Pub/Sub for text) — APPROVED
- Token granularity: per-line/chunk — APPROVED
- Pub/Sub: allowed — APPROVED; accept ephemeral text; durability via final state
- Event schema stability: `id`, `type`, `ts`, `data` — APPROVED
