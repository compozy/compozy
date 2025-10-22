# Executions Streaming (/stream) — Implementation Task Summary

## Relevant Files

### Core Implementation Files

- `engine/infra/server/router/sse.go` – SSE helpers (headers, write, heartbeat)
- `engine/workflow/router/stream.go` – Workflow streaming endpoint
- `engine/agent/router/stream.go` – Agent streaming endpoint (structured/text)
- `engine/task/router/stream.go` – Task streaming endpoint (structured/text)
- `engine/task/uc/exec_task.go` – Worker emission hook for simulated text chunks (Phase 1)
- `engine/worker/...` (Temporal workflow code) – Add SetQueryHandler for workflow stream state (if applicable)

### Integration Points

- `engine/infra/redis` – Redis client used for Pub/Sub
- `engine/infra/server/routes` – Route registration for `/stream` endpoints
- `engine/workflow/repo.go` & `engine/infra/postgres/*` – Workflow state lookups (fallbacks, metrics)
- `engine/task/repo.go` & `engine/infra/postgres/taskrepo.go` – Task state lookups

### Documentation Files

- `docs/content/docs/api/*` – API pages (workflows, agents, tasks, overview)
- `docs/content/docs/schema/execution-stream-events.mdx` – Event catalog
- Swagger (`docs/swagger.yaml` generation via handlers)

### Examples (if applicable)

- `examples/stream/sse-browser/*`
- `examples/stream/sse-node/*`

## Tasks

- [ ] 1.0 SSE Helper Utilities (S)
- [ ] 2.0 Workflow Stream Endpoint + Temporal Query Wiring (M)
- [ ] 3.0 Agent Stream Endpoint (structured + text) (M)
- [ ] 4.0 Task Stream Endpoint (structured + text) (M)
- [ ] 5.0 Worker Publisher for Text Chunks (Phase 1 shim) (M)
- [ ] 6.0 Swagger + Docs Updates (S)
- [ ] 7.0 Examples: Browser + Node consumers (S)

Notes on sizing:

- S = Small (≤ half-day)
- M = Medium (1–2 days)
- L = Large (3+ days)

## Task Design Rules

- Each parent task is a closed deliverable: independently shippable and reviewable
- Do not split one deliverable across multiple parent tasks; avoid cross-task coupling
- Each parent task must include unit test subtasks derived from `_tests.md` for this feature
- Each generated `/_task_<num>.md` must contain explicit Deliverables and Tests sections

## Execution Plan

- Critical Path: 1.0 → 2.0 → 3.0/4.0 (parallel) → 5.0 → 6.0 → 7.0
- Parallel Track A (after 1.0): 2.0
- Parallel Track B (after 1.0): 3.0 and 4.0 can proceed in parallel
- Parallel Track C (after 2.0/3.0/4.0 APIs are shaped): 5.0 publisher & tests
- Docs and examples (6.0, 7.0) can run partially in parallel after endpoints stabilize

Notes

- All runtime code MUST use `logger.FromContext(ctx)` and `config.FromContext(ctx)`
- Run `make fmt && make lint && make test` before marking any task as completed

## Batch Plan (Grouped Commits)

- [ ] Batch 1 — Foundations: 1.0, 2.0
- [ ] Batch 2 — Endpoints: 3.0, 4.0
- [ ] Batch 3 — Emission & Tests: 5.0
- [ ] Batch 4 — Docs & Examples: 6.0, 7.0
