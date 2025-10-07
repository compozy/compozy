---
status: pending
parallelizable: false
blocked_by: ["0.0"]
---

<task_context>
<domain>engine/knowledge</domain>
<type>implementation|testing</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
<unblocks>"2.0","3.0","4.0","5.0","7.0","8.0"</unblocks>
</task_context>

# Task 1.0: Knowledge Domain Scaffolding

## Overview
Create `engine/knowledge` root with config types (Embedder, VectorDB, KnowledgeBase), validation, and resource registration hooks used by the rest of the feature.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Define config structs per Tech Spec (IDs, sources, chunking, retrieval, metadata).
- Implement validators with precise error messages; forbid unsupported source types.
- Ensure all runtime paths derive `logger` and `config` via context; no globals.
- Provide resource registration compatible with project/workflow autoload.
- Run `make fmt && make lint && make test` before completion.
</requirements>

## Subtasks
- [ ] 1.1 Create `engine/knowledge/config.go` and `engine/knowledge/errors.go`
- [ ] 1.2 Implement validation (chunk size/overlap/strategy, top_k/min_score defaults)
- [ ] 1.3 Add unit tests `engine/knowledge/config_test.go`
  - Should validate missing embedder/vector refs
  - Should reject invalid chunking and source kinds
  - Should normalize defaults

## Sequencing
- Blocked by: 0.0
- Unblocks: 2.0, 3.0, 4.0, 5.0, 7.0, 8.0
- Parallelizable: No (foundation)

## Implementation Details
Follow `_techspec.md` component outlines and field names. Keep configuration free of provider details; adapters handle provider specifics.

### Relevant Files
- `engine/knowledge/config.go`
- `engine/knowledge/errors.go`

### Dependent Files
- `engine/knowledge/embedder/*`
- `engine/knowledge/vectordb/*`

## Success Criteria
- Config and validation compile; unit tests pass; clear error messages per standard.
