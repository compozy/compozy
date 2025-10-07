status: completed
parallelizable: false
blocked_by: ["1.0"]

---

<task_context>
<domain>engine/knowledge/vectordb</domain>
<type>implementation|testing</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>database</dependencies>
<unblocks>"5.0","6.0","12.0","15.0"</unblocks>
</task_context>

# Task 3.0: Vector DB Adapters

## Overview

Implement pgvector, qdrant, and in‑memory vector stores with schema/index management (`ensure_index`) and nearest‑neighbor queries.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- DSN/host credentials must come from `config.FromContext(ctx)`.
- Provide minimal CRUD needed by ingestion/retrieval; support cosine similarity.
- In‑memory adapter for deterministic unit tests.
- Unit tests must not start containers; use in‑memory stub.
- Run `make fmt && make lint && make test` before completion.
</requirements>

## Subtasks

- [x] 3.1 Create `engine/knowledge/vectordb/interface.go` and concrete adapters
- [x] 3.2 Implement `ensure_index` and dimension checks
- [x] 3.3 Unit tests (in-memory): upsert/query; dimension mismatch errors; DSN parse sanity for pgvector

## Sequencing

- Blocked by: 1.0
- Unblocks: 5.0, 6.0, 12.0, 15.0
- Parallelizable: No

## Implementation Details

Keep metadata payloads inline with text; avoid external blob stores in MVP.

### Relevant Files

- `engine/knowledge/vectordb/*`

### Dependent Files

- `engine/knowledge/ingest/*`
- `engine/knowledge/retriever/*`

## Success Criteria

- Adapters compile; in‑memory unit tests pass; pgvector/qdrant configs validated.
