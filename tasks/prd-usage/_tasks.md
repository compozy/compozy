# LLM Usage Reporting Implementation Task Summary

## Relevant Files

### Core Implementation Files

- `engine/infra/postgres/migrations/20251015090000_create_execution_llm_usage.sql` – Goose migration defining `execution_llm_usage`
- `engine/infra/postgres/usage_repo.go` – Repository for persisting usage rows
- `engine/llm/adapter/langchain_adapter.go` – LangChain adapter changes to populate usage
- `engine/llm/usage/*` – New usage collector package (aggregation, interfaces)
- `engine/llm/orchestrator/loop.go` – Hook usage collector during orchestrator runs
- `engine/task/directexec/direct_executor.go` – Propagate execution metadata for direct tasks
- `engine/infra/server/router/*.go` – Agent/task/workflow routers returning usage summaries
- `infra/monitoring/*` & `cluster/grafana/alerts/llm-usage-alerts.yaml` – Metrics registration and alert rules

### Integration Points

- `engine/llm/orchestrator` ↔ `engine/infra/postgres` – Collector to repository writes
- `engine/task/directexec` – Ensures metadata for direct executions
- `engine/infra/server/router` – API responses and CLI integration
- `infra/monitoring` / Grafana dashboards – Metrics and alert coverage

### Documentation Files

- `docs/api/executions.mdx`
- `docs/api/agents.mdx`
- `docs/api/tasks.mdx`
- `docs/how-to/monitor-usage.mdx`
- `docs/reference/schemas/execution-usage.mdx`
- `docs/cli/executions.mdx`
- `docs/concepts/observability.mdx`
- `docs/source.config.ts`
- `schemas/execution-usage.json`

### Examples (if applicable)

- `examples/usage-reporting/workflow-summary/*`
- `examples/usage-reporting/task-direct-exec/*`
- `examples/usage-reporting/agent-sync/*`

## Tasks

- [ ] 1.0 Database Migration & Usage Repository (M)
- [ ] 2.0 LangChain Adapter Usage Extraction (S)
- [ ] 3.0 Usage Collector & Orchestrator Integration (L)
- [ ] 4.0 API & CLI Usage Exposure (M)
- [ ] 5.0 Observability Metrics & Alerts (M)
- [ ] 6.0 Documentation, Schema & Examples (M)

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

- **Critical Path:** 1.0 → 2.0 → 3.0 → 4.0 → 6.0
- **Parallel Track A:** 5.0 (can start after 3.0 alongside 4.0)
- **Parallel Track B:** None (remaining tasks depend on critical path completion)

Notes

- All runtime code MUST use `logger.FromContext(ctx)` and `config.FromContext(ctx)`
- Run `make fmt && make lint && make test` before marking any task as completed

## Batch Plan (Grouped Commits)

- [ ] Batch 1 — Core Persistence & Adapter: 1.0, 2.0
- [ ] Batch 2 — Collector & Surface: 3.0, 4.0
- [ ] Batch 3 — Observability & Docs: 5.0, 6.0
