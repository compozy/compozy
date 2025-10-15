# LLM Usage Reporting

## Summary

- Capture and persist Large Language Model (LLM) usage metrics for every Compozy execution (agent, task, workflow) so users can reconcile spend and monitor cost hot-spots.
- Enrich HTTP execution endpoints with token usage summaries while keeping historical data in PostgreSQL for future analytics.
- Reuse existing orchestrator and telemetry plumbing without breaking backwards compatibility for current API consumers.

## Goals

- Surface prompt, completion, total token counts (including reasoning/cached tokens when available) per execution in `/executions/*` responses.
- Persist execution-level usage in PostgreSQL with referential links to workflow/task executions.
- Provide Prometheus metrics and dashboards for LLM usage.
- Expose collected data through a dedicated domain service to avoid duplicating logic across routers, worker flows, and CLI integrations.

## Non-Goals

- Detailed per-message logs beyond aggregated token counts (future extension).
- Realtime billing integration with external providers.
- Retrofitting historical executions (optional backfill noted in rollout).

## Current State (2025-10-15)

- `engine/llm/adapter/langchain_adapter.go:559` converts `llms.ContentResponse` to `LLMResponse` but drops `GenerationInfo`, leaving `response.Usage` empty.
- `engine/llm/orchestrator/loop.go:494` already derives contextual usage metrics (prompt/completion totals) but has no persistence hooks; data is only emitted in telemetry events.
- API DTOs (`engine/agent/router/dto.go:85`, `engine/task/router/exec.go:321`, `engine/workflow/router/execs.go:1`) do not expose usage fields.
- Persistence layer (`task_states`, `workflow_states` migrations) lacks columns or companion tables for LLM usage; repositories (`engine/infra/postgres/taskrepo.go`, `workflowrepo.go`) read/write task/workflow state only.
- Monitoring stack exports latency/error counters but no token-specific signals (`engine/infra/monitoring/execution_metrics.go`).

## External Capability Reference

- LangChainGo populates `ContentChoice.GenerationInfo` with `CompletionTokens`, `PromptTokens`, `TotalTokens`, and reasoning token counts when using OpenAI models, enabling downstream consumers to read token usage metadata. citeturn3search0
- OpenAI chat responses include a `usage` object (`prompt_tokens`, `completion_tokens`, `total_tokens`) for each call; this metadata is delivered both for sync responses and as a trailing chunk when streaming with `stream_options.include_usage`. citeturn0search0turn0search10

## Proposed Architecture

### Overview

```
LangChainGo Model -> LangChainAdapter (extract usage) -> llmusage.Collector -> llmusage.Repository -> Postgres
                                                                |
                                           Execution routers load & embed usage DTOs
```

1. **Collection**
   - Extend `LangChainAdapter.convertResponse` to parse `choice.GenerationInfo` and populate `LLMResponse.Usage` (prompt/completion/total, reasoning, cached tokens).
   - Ensure OpenAI-specific adapters request `stream_options.include_usage=true` so streaming executions receive final usage chunks.
2. **Aggregation**
   - Introduce `engine/llm/usage` package with:
     - `Collector` interface to accept raw `LLMUsageEvent` (execution IDs, model, provider, token counts, timestamp).
     - `Aggregator` to accumulate per-task execution totals during orchestrator loops. Hook into `conversationLoop.recordLLMResponse` and finalize on loop completion.
     - Context helpers to carry `task_exec_id` / `workflow_exec_id` from caller metadata into the orchestrator (populate `orchestrator.RunMetadata` in `engine/llm/service.go:360` and `task/directexec/direct_executor.go`).
3. **Persistence**
   - Repository storing per-execution totals in PostgreSQL (see Data Model).
   - Optional per-call event table to enable detailed analytics later (flagged as stretch).
4. **Exposure**
   - APIs augment execution payloads with usage summaries.
   - CLI/SDKs fetch new fields without breaking older clients (fields are additive).
5. **Observability**
   - Emit Prometheus counters for prompt/completion tokens and cost estimations (model-based) via new `monitoring/usage_metrics.go`.

## Data Model & Schema

### Tables

1. **`execution_llm_usage`** (primary target)  
   | Column | Type | Notes |
   |--------|------|-------|
   | `id` | `bigserial` | PK |
   | `workflow_exec_id` | `text` | FK → `workflow_states.workflow_exec_id`, nullable for direct agent/task runs |
   | `task_exec_id` | `text` | FK → `task_states.task_exec_id`, nullable for workflow aggregate |
   | `component` | `text` | enum-like (`workflow`, `task`, `agent`) |
   | `agent_id` | `text` | optional |
   | `model` | `text` | normalized provider model name |
   | `provider` | `text` | e.g., `openai` |
   | `prompt_tokens` | `integer` | |
   | `completion_tokens` | `integer` | |
   | `total_tokens` | `integer` | |
   | `reasoning_tokens` | `integer` | nullable |
   | `cached_prompt_tokens` | `integer` | nullable |
   | `input_audio_tokens` | `integer` | nullable |
   | `output_audio_tokens` | `integer` | nullable |
   | `created_at` | `timestamptz` | default now() |
   | `updated_at` | `timestamptz` | default now() |
   | Unique constraint on (`task_exec_id`, `component`) to prevent duplicates.

2. **`workflow_llm_usage_totals`** _(optional aggregate table)_
   - Precomputed sums per `workflow_exec_id` for fast queries; maintained via triggers or upsert in orchestrator finalization.

### Indexing & Retention

- Index on `workflow_exec_id`, `task_exec_id` to support execution fetches.
- Composite index on `(component, created_at)` for reporting.
- Retention policy configurable (default 180 days) with scheduled cleanup job.

## API Surface Changes

### Agent Executions

- `AgentExecSyncResponse` (`engine/agent/router/dto.go:104`): add `usage` object with token counts.
- `ExecutionStatusDTO` (`engine/agent/router/exec.go:49`): embed `UsageSummary`.

### Task Executions

- `TaskExecSyncResponse` & `TaskExecutionStatusDTO` (`engine/task/router/exec.go:321`): add `usage`.
- Async status fetch `/executions/tasks/{id}` returns usage once persisted.

### Workflow Executions

- `/executions/workflows/{exec_id}` response to include aggregated `usage` alongside task tree; workflow-level table supports this.

### JSON Shape (example)

```json
{
  "exec_id": "2Z4PVTL6K27XVT4A3NPKMDD5BG",
  "status": "completed",
  "usage": {
    "model": "gpt-4o-mini",
    "provider": "openai",
    "prompt_tokens": 812,
    "completion_tokens": 265,
    "total_tokens": 1077,
    "reasoning_tokens": 0,
    "cached_prompt_tokens": 120
  },
  "output": {...}
}
```

### Contract Notes

- All new fields are nullable; absence implies usage not recorded (e.g., legacy executions).
- CLI should update serialization helpers to display usage when present.

## Persistence & Wiring

- Extend `DirectExecutor` metadata to carry execution IDs into LLM service (`engine/task/directexec/direct_executor.go:151`).
- Modify `engine/llm/service.go:360` to set `RunMetadata.WorkflowID`/`ExecutionID`.
- `llmusage.Repository` invoked after each LLM response; aggregator flushes totals when loop terminates successfully or with error (still record usage to diagnostic).
- Provide repository implementations in `engine/infra/postgres/usage_repo.go` with migrations.

## Observability & Analytics

- New Prometheus counters:
  - `compozy_llm_prompt_tokens_total{component,model,provider}`
  - `compozy_llm_completion_tokens_total{component,model,provider}`
  - `compozy_llm_usage_events_total{status}`
- Grafana dashboard `dashboards/llm-usage.json` summarizing usage per workflow, top agents, cost estimator (tokens × configurable rate).
- Alerts on abnormal spikes (e.g., >3× rolling average for a workflow within 1h).

## Implementation Plan

1. **Adapter & Collector (Backend)**
   - Parse `GenerationInfo` in `LangChainAdapter` and populate `LLMResponse.Usage`.
   - Implement `llmusage` package (domain structs, collector, aggregator).
   - Wire collector into orchestrator loop and direct executor metadata flow.
2. **Persistence Layer**
   - Add goose migration for `execution_llm_usage`.
   - Implement Postgres repository + integration tests.
3. **API Enhancements**
   - Update DTOs and response builders for agent/task/workflow routers.
   - Extend CLI client models/tests.
4. **Metrics & Dashboard**
   - Instrument Prometheus metrics.
   - Add Grafana dashboard & alerts.
5. **Rollout & Backfill**
   - Deploy migration.
   - Enable feature flag to record usage.
   - Optional backfill: replay recent executions to populate usage table.
   - Update documentation & changelog.
