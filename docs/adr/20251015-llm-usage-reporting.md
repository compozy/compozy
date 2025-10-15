# ADR 2025-10-15 — LLM Usage Reporting Pipeline

## Status

Accepted

## Context

We need to persist execution-level LLM usage for workflows, tasks, and direct agent runs, and surface the data through APIs and the CLI. The orchestrator already exposes `LLMResponse.Usage`, repositories for the `execution_llm_usage` table exist, and direct/task executions manage execution metadata locally.

## Decision

1. Introduce a `usage.Collector` that aggregates `LLMResponse.Usage` snapshots and upserts totals via the existing Postgres repository. Collectors are bound to a `context.Context` so orchestrator hooks can record usage without new plumbing.
2. Attach collectors at execution edges:
   - Direct executions (`directexec`) create a collector per task run.
   - Workflow activities (`ExecuteBasic`, `ExecuteSubtask`) initialise collectors using the persisted task state metadata.
3. The orchestrator records token usage during `recordLLMResponse`. Finalization happens when executions reach a terminal status (success/failure/panic).
4. Workflow/agent/task routers fetch usage rows through a shared helper and expose a nullable `usage` block in DTOs and CLI outputs.
5. Documentation references this ADR for traceability.

## Consequences

- Aggregation occurs once per execution, ensuring retries overwrite prior rows through `Upsert`.
- Context-first collectors keep orchestration decoupled from persistence wiring.
- API consumers receive additive `usage` data without breaking existing contracts.
- Future work (metrics/alerts) can reuse the collector and repository without new instrumentation paths.
