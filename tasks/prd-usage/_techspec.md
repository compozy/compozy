# Technical Specification Template

## Executive Summary

The LLM Usage Reporting feature introduces execution-level token tracking across workflows, direct task runs, and agent executions. We will enhance the LangChain adapter to populate usage metadata, add an `llmusage` package that collects and aggregates usage during orchestrator runs, persist the data in a new `execution_llm_usage` table, and expose the results through existing `/executions/*` APIs. Observability updates will emit counters and health metrics so operations can detect gaps quickly. The design prioritizes forward-only ingest with minimal coupling while keeping hooks for future backfill or analytics phases. For context, see the product requirements and architectural sketch in [`USAGE_FEATURE.md`](../../USAGE_FEATURE.md).

## System Architecture

```
LangChainGo Model
        │
        ▼
engine/llm/adapter (populate LLMResponse.Usage)
        │
        ▼
engine/llm/usage Collector  ─────┐
        │                        │
        ▼                        │
engine/infra/postgres Repository │
        │                        │
        ▼                        │
   execution_llm_usage table     │
        │                        │
        ▼                        │
engine/infra/server/router ◄─────┘
        │
        ▼
/executions/* API + CLI responses
```

### Domain Placement

- `engine/llm/adapter` – extend LangChain adapter to extract usage.
- `engine/llm/orchestrator` – invoke usage collector, pass execution metadata.
- `engine/llm/usage` (new) – encapsulate collection, aggregation, and reporting logic.
- `engine/infra/postgres` – add repository for `execution_llm_usage` plus migration.
- `engine/infra/server/router` – update task/agent/workflow routers to embed usage DTOs.
- `engine/infra/monitoring` – register usage counters and alerts.
- `pkg/logger`, `pkg/config` – continue using context accessors per rules.

### Component Overview

- **LangChain Adapter**: Normalizes provider responses into `LLMResponse.Usage` so downstream code sees consistent token counts.
- **Usage Collector (engine/llm/usage)**: Stateful helper capturing run-scoped usage events; flushes totals at loop completion and on errors. Collector performs an upsert on every terminal status transition (success, failure, timeout) so retried executions overwrite prior usage rows.
- **Postgres Repository**: Persists usage rows linked to workflow/task exec IDs; enforces FK integrity and idempotent upserts.
- **Execution Routers**: Enrich response payloads with usage summaries sourced from repository.
- **Monitoring Layer**: Exposes counters for prompt/completion totals, ingestion failures, and latency.
- **CLI/SDK Surface**: No code changes beyond relying on existing response schema.

## Implementation Design

### Core Interfaces

```go
// UsageCollector captures token usage for an execution lifecycle.
type UsageCollector interface {
    RecordResponse(ctx context.Context, execMeta ExecMetadata, usage UsageSnapshot)
    Finalize(ctx context.Context, execMeta ExecMetadata, status ExecutionStatus) error
}

// UsageRepository persists and retrieves usage summaries.
type UsageRepository interface {
    Upsert(ctx context.Context, row *UsageRow) error
    GetByTaskExecID(ctx context.Context, id core.ID) (*UsageRow, error)
    GetByWorkflowExecID(ctx context.Context, id core.ID) (*UsageRow, error)
}
```

## Planning Artifacts (Must Be Generated With Tech Spec)

- Docs Plan: `tasks/prd-usage/_docs.md`
- Examples Plan: `tasks/prd-usage/_examples.md`
- Tests Plan: `tasks/prd-usage/_tests.md`

### Data Models

- **UsageRow (Go struct)**
  - `WorkflowExecID *core.ID`
  - `TaskExecID *core.ID`
  - `Component core.ComponentType` (`workflow`, `task`, `agent`)
  - `AgentID *string`
  - `Provider string`
  - `Model string`
  - `PromptTokens int`
  - `CompletionTokens int`
  - `TotalTokens int`
  - `ReasoningTokens *int`
  - `CachedPromptTokens *int`
  - `InputAudioTokens *int`
  - `OutputAudioTokens *int`
  - `CreatedAt time.Time`
  - `UpdatedAt time.Time`
- **Status-aware Persistence**: the collector must upsert usage rows whenever executions reach a terminal status (success, failure, timeout) so retried runs overwrite previous values and reflect the latest state.
- **SQL Migration**: add goose migration `20251015090000_create_execution_llm_usage.sql` that creates `execution_llm_usage` with FKs to `workflow_states.workflow_exec_id` and `task_states.task_exec_id`, unique constraint on `(task_exec_id, component)`, and indexes on `workflow_exec_id`, `task_exec_id`, and `(component, created_at)` for reporting (per `USAGE_FEATURE.md`).
- **Retention Hooks**: document default 180-day retention in operations runbook; future retention/backfill tasks can add `deleted_at` or partitioning as part of Phase 2.
- **API DTO Extension**: add `UsageSummary` struct to agent/task/workflow responses containing usage fields (nullable) and map provider-specific extras into optional fields.

| Column                 | Type          | Notes                                                             |
| ---------------------- | ------------- | ----------------------------------------------------------------- |
| `id`                   | `bigserial`   | Primary key                                                       |
| `workflow_exec_id`     | `text`        | FK → `workflow_states.workflow_exec_id`, nullable for direct runs |
| `task_exec_id`         | `text`        | FK → `task_states.task_exec_id`, nullable for workflow aggregate  |
| `component`            | `text`        | Execution scope (`workflow`, `task`, `agent`)                     |
| `agent_id`             | `text`        | Optional agent identifier                                         |
| `provider`             | `text`        | Provider name (e.g., `openai`)                                    |
| `model`                | `text`        | Resolved model identifier                                         |
| `prompt_tokens`        | `integer`     | Prompt token count                                                |
| `completion_tokens`    | `integer`     | Completion token count                                            |
| `total_tokens`         | `integer`     | Prompt + completion (fallback)                                    |
| `reasoning_tokens`     | `integer`     | Nullable reasoning tokens                                         |
| `cached_prompt_tokens` | `integer`     | Nullable cached tokens                                            |
| `input_audio_tokens`   | `integer`     | Optional provider-specific field                                  |
| `output_audio_tokens`  | `integer`     | Optional provider-specific field                                  |
| `created_at`           | `timestamptz` | Default `now()`                                                   |
| `updated_at`           | `timestamptz` | Default `now()`                                                   |

### API Endpoints

- `GET /api/v0/executions/workflows/:exec_id`
- `GET /api/v0/executions/tasks/:exec_id`
- `GET /api/v0/executions/agents/:exec_id`
  - Each now returns `"usage": { ... }` object with token counts and metadata reflecting the execution's latest terminal status (failed executions still emit whatever usage was recorded).
- **Response Example**

```json
{
  "exec_id": "2Z4PVTL6K27XVT4A3NPKMDD5BG",
  "status": "failed",
  "usage": {
    "provider": "openai",
    "model": "gpt-4o-mini",
    "prompt_tokens": 812,
    "completion_tokens": 265,
    "total_tokens": 1077,
    "reasoning_tokens": 0,
    "cached_prompt_tokens": 120
  }
}
```

## Integration Points

- External LLM providers already integrated via LangChainGo. No new external API calls; rely on existing responses. Ensure streaming requests pass `stream_options.include_usage=true` where available.
- `engine/task/directexec` and `engine/agent/exec` must propagate execution metadata (IDs, component) into the collector so retries and async completions update usage rows correctly.
- Observability stack (Prometheus + Grafana) consumes new metrics/alerts; coordinate with infra team for deployment of `llm-usage-alerts.yaml` and dashboard panels.

## Impact Analysis

| Affected Component                        | Type of Impact       | Description & Risk Level                                                                                       | Required Action                          |
| ----------------------------------------- | -------------------- | -------------------------------------------------------------------------------------------------------------- | ---------------------------------------- |
| `engine/llm/adapter/langchain_adapter.go` | Logic Update         | Parse `GenerationInfo` usage into `LLMResponse`. Low risk; requires provider tests.                            | Update adapter + add unit tests          |
| `engine/llm/orchestrator`                 | Behavior Change      | Hook collector to loop lifecycle, propagate exec metadata. Medium risk due to concurrency.                     | Add usage collector with guarded context |
| `engine/infra/postgres`                   | Schema Change        | New table + repo for usage rows. Medium risk; run migration tests.                                             | Create goose migration                   |
| `engine/agent/task/workflow` routers      | API Extension        | Add `usage` field to DTOs; backwards-compatible. Low risk.                                                     | Update DTOs + JSON schema docs           |
| `infra/monitoring`                        | Metrics Addition     | Counters and alerts for usage ingestion. Low risk.                                                             | Register metrics + update dashboards     |
| `engine/task/directexec`                  | Metadata Propagation | Ensure direct task runs set execution metadata so collector can upsert usage on retries/timeouts. Medium risk. | Hook metadata injection + tests          |

## Testing Approach

### Unit Tests

- Adapter: ensure `convertResponse` populates `Usage` for responses with/without metadata.
- Usage collector: verify aggregation logic across iterations and finalize behavior.
- Repository: test upsert semantics and FK enforcement using pgxmock.
- Router DTO builders: confirm usage is attached when repository returns data.

### Integration Tests

- End-to-end workflow execution with mocked LLM returning usage -> assert DB row + API response.
- Failure path (usage missing) -> ensure null fields and warning log.
- Metrics emission smoke test (via `infra/monitoring` test harness).
- Follow `.cursor/rules/test-standards.mdc`: use `t.Run("Should …")` naming, place integration suites under `test/integration/usage/`, avoid redundant low-value tests, and rely on testify for assertions/mocks.

## Development Sequencing

### Build Order

1. **LangChain Adapter Updates** – prerequisite to capture usage metadata.
2. **Usage Collector Package** – depends on adapter shape; implement aggregator and context helpers, including status-aware upsert semantics for retries and failures.
3. **Postgres Migration & Repository** – enable persistence after collector can produce rows.
4. **Router/API Extensions** – surface data to clients once persistence works.
5. **Monitoring Hooks** – register counters once ingestion path is ready.
6. **Docs/Tests** – finalize documentation, examples, and full test suite .

### Technical Dependencies

- Database migration approval and staging window.
- Access to provider environments to test streaming usage metadata.
- Coordination with observability team for dashboard updates (Phase 2 optional).

## Monitoring & Observability

- **Metrics**: `compozy_llm_prompt_tokens_total`, `compozy_llm_completion_tokens_total`, `compozy_llm_usage_events_total`, `compozy_llm_usage_failures_total`, and `compozy_llm_usage_latency_seconds` histogram.
- **Alerting**: add `cluster/grafana/alerts/llm-usage-alerts.yaml` with rules:
  - Critical: failure rate >1% for 15 minutes (`compozy_llm_usage_failures_total` derivative).
  - Warning: workflow token usage exceeds 3× 24h rolling average.
  - Warning: zero usage logged for known LLM workflows for 30 minutes.
- **Dashboards**: extend LLM operations board (Grafana) with coverage %, failure trends, latency histogram, and top workflows by tokens.
- **Logs**: structured warning when usage metadata unavailable; error logs on repository failures with execution IDs for triage.
- **Runbooks**: update observability runbook to document metrics interpretation, alert response steps, and escalation path.

## Technical Considerations

### Key Decisions

- Maintain forward-only ingest (no initial backfill) to minimize scope; revisit in Phase 2.
- Store execution usage in dedicated table instead of embedding in `workflow_states` to avoid bloating existing rows and to simplify retention.
- Use context-based usage collector to remain compliant with `.cursor/rules/architecture.mdc` (avoid singletons, respect context propagation).
- Aggregate per execution rather than per message/tool result to keep storage and API responses lean; revisit more granular logging only if downstream billing requires it.

### Known Risks

- Provider usage metadata may be missing or inconsistent – mitigation includes null defaults, logging, and alerting.
- Increased load on primary database – monitor table size and evaluate partitioning/retention in later phase.
- Potential latency impact during persistence – plan asynchronous/queued writes if synchronous insert exceeds thresholds.

### Special Requirements

- Performance: keep additional processing under 5% p95 latency increase on execution APIs.
- Security: ensure database credentials pulled from context-configured pool; no new secrets introduced.

### Standards Compliance

- Adheres to `.cursor/rules/architecture.mdc` (context-first, modular design).
- Follows `.cursor/rules/go-coding-standards.mdc` for Go code style and function length limits.
- Uses `.cursor/rules/test-standards.mdc` for test naming and structure.
- Respects `.cursor/rules/no-linebreaks.mdc` formatting guidance.

## Change Control & Documentation

- **Living Documents**: update `tasks/prd-usage/_prd.md` after implementation milestones; include change log entries referencing commits.
- **Docs & OpenAPI**: after implementation, regenerate OpenAPI spec to include `usage` object, update CLI/API docs per `_docs.md`
