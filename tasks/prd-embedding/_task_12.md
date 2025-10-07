---
status: pending
parallelizable: true
blocked_by: ["5.0","6.0"]
---

<task_context>
<domain>engine/knowledge|engine/infra/server|engine/llm</domain>
<type>implementation|testing</type>
<scope>observability</scope>
<complexity>low</complexity>
<dependencies>http_server|external_apis|database</dependencies>
<unblocks>"15.0"</unblocks>
</task_context>

# Task 12.0: Observability

## Overview
Add metrics, structured logs, and tracing for ingestion and retrieval; label metrics by `kb_id` where applicable.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Metrics: `knowledge_ingest_duration_seconds`, `knowledge_chunks_total`, `knowledge_query_latency_seconds`.
- Structured logs for start/finish/failure with `kb_id` context.
- Spans around embedder/vector operations with provider/model attributes.
- Unit tests assert counters/logs/spans presence where feasible.
- Run `make fmt && make lint && make test` before completion.
</requirements>

## Subtasks
- [ ] 12.1 Add counters/histograms and labels
- [ ] 12.2 Add structured logs and spans in hot paths
- [ ] 12.3 Augment unit tests in ingest/retriever to assert observability hooks

## Sequencing
- Blocked by: 5.0, 6.0
- Unblocks: 15.0
- Parallelizable: Yes

## Implementation Details
Use existing observability facilities in repo; avoid introducing new deps.

### Relevant Files
- `engine/knowledge/*`
- `engine/infra/server/router/knowledge/*`

### Dependent Files
- `test/integration/knowledge/*`

## Success Criteria
- Metrics/logs/spans verified by unit tests; naming matches docs.
