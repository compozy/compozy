status: completed
parallelizable: true
blocked_by: ["1.0"]

---

<task_context>
<domain>engine/knowledge/chunk</domain>
<type>implementation|testing</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies></dependencies>
<unblocks>"5.0"</unblocks>
</task_context>

# Task 4.0: Chunking & Preprocess

## Overview

Implement chunking strategies (e.g., recursive splitter), deduplication, HTML stripping, and content hashing for idempotent ingestion.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Provide deterministic chunk IDs; support size/overlap defaults.
- Preprocess pipeline: optional HTML strip, newline normalization, dedupe.
- Unit tests must cover chunk boundaries and dedupe behavior.
- Run `make fmt && make lint && make test` before completion.
</requirements>

## Subtasks

- [x] 4.1 Implement `engine/knowledge/chunk/*` (splitters, preprocess)
- [x] 4.2 Unit tests `engine/knowledge/ingest_test.go` (chunk/dedupe sections)
  - Should chunk per strategy/size/overlap; stable IDs
  - Should deduplicate by content hash

## Sequencing

- Blocked by: 1.0
- Unblocks: 5.0
- Parallelizable: Yes

## Implementation Details

Keep the interface small; return slices of chunks with text and metadata ready for embedding.

### Relevant Files

- `engine/knowledge/chunk/*`

### Dependent Files

- `engine/knowledge/ingest/*`

## Success Criteria

- Deterministic chunking and dedupe verified by unit tests.
