---
status: pending
parallelizable: false
blocked_by: ["2.0", "3.0", "4.0"]
---

<task_context>
<domain>engine/knowledge/ingest</domain>
<type>implementation|testing</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>external_apis|database</dependencies>
<unblocks>"10.0","12.0","15.0"</unblocks>
</task_context>

# Task 5.0: Ingestion Pipeline

## Overview

Build enumerate → embed → persist pipeline supporting `markdown_glob` and `pdf_url` sources (cloud/media optional). Handle batching, retries, and idempotency.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Enumerate sources; compute content hashes for dedupe.
- Batch embeddings per provider; surface structured errors.
- Persist vectors with inline text and metadata; no external blobs.
- Unit tests cover batching, idempotency, and metadata persistence.
- Run `make fmt && make lint && make test` before completion.
</requirements>

## Subtasks

- [ ] 5.1 Implement source enumeration and hashing
- [ ] 5.2 Wire embedder + vectordb adapters; implement retries/backoff
- [ ] 5.3 Unit tests `engine/knowledge/ingest_test.go`
  - Should batch by limit; propagate provider errors
  - Should persist inline payloads; re‑ingest is idempotent

## Sequencing

- Blocked by: 2.0, 3.0, 4.0
- Unblocks: 10.0, 12.0, 15.0
- Parallelizable: No

## Implementation Details

Use context for cancellation and logging; include counters for chunks embedded and persisted.

### Relevant Files

- `engine/knowledge/ingest/*`

### Dependent Files

- `engine/infra/server/router/knowledge/*`

## Success Criteria

- Pipeline compiles; unit tests pass; idempotency verified.
