status: completed
parallelizable: false
blocked_by: ["2.0", "3.0", "7.0"]

---

<task_context>
<domain>engine/knowledge/retriever</domain>
<type>implementation|testing</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>external_apis|database</dependencies>
<unblocks>"9.0","10.0"</unblocks>
</task_context>

# Task 6.0: Retrieval Service

## Overview

Implement dense similarity retrieval with `top_k`/`min_score`, deterministic ordering, and optional token‑budget trimming for prompt injection.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Expose a simple service interface resolving knowledge bindings to providers/stores.
- Stable sort and tie‑breaking; respect `min_score` filter.
- Unit tests for ordering, thresholds, and trimming.
- Run `make fmt && make lint && make test` before completion.
</requirements>

## Subtasks

- [x] 6.1 Implement retriever API and scoring
- [x] 6.2 Add max token trimming helper for injection
- [x] 6.3 Unit tests `engine/knowledge/retriever_test.go`
  - Should respect `top_k` and `min_score`; stable ordering
  - Should trim by `max_tokens`

## Sequencing

- Blocked by: 2.0, 3.0, 7.0
- Unblocks: 9.0, 10.0
- Parallelizable: No

## Implementation Details

Rely on vector store for similarity; avoid rerankers in MVP.

### Relevant Files

- `engine/knowledge/retriever/*`
- `engine/knowledge/service.go`

### Dependent Files

- `engine/llm/orchestrator/*`

## Success Criteria

- Retrieval returns correctly filtered/ordered results; tests pass.

## Outcome

- Delivered `retriever.Service` with stable ordering, min-score filtering, metadata cloning, and token budget trimming.
- Added exhaustive unit tests for ordering, filtering, trimming, and retry-store interactions.
- Ensured ingest pipeline delete-on-replace integrates with retrieval metadata contract.
- Verified via `make fmt`, `make lint`, and `make test`.
