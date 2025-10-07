---
status: pending
parallelizable: false
blocked_by: ["1.0","0.0"]
---

<task_context>
<domain>engine/knowledge/embedder</domain>
<type>implementation|testing</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>external_apis</dependencies>
<unblocks>"5.0","6.0"</unblocks>
</task_context>

# Task 2.0: Embedder Adapters

## Overview
Implement adapters wrapping LangChainGo embeddings (OpenAI, Vertex, local) with input normalization, batching, and error propagation.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Provide `EmbedDocuments` and `EmbedQuery` via a slim interface.
- Validate dimensions/batch sizes per provider; surface actionable errors.
- No globals; provider config read via `config.FromContext(ctx)`.
- Include unit tests; mock external APIs.
- Run `make fmt && make lint && make test` before completion.
</requirements>

## Subtasks
- [ ] 2.1 Create `engine/knowledge/embedder/interface.go` and provider files
- [ ] 2.2 Implement normalization (strip newlines if configured)
- [ ] 2.3 Unit tests (adapter-focused)
  - Should batch by provider limit; propagate errors with context
  - Should normalize input per config

## Sequencing
- Blocked by: 1.0, 0.0
- Unblocks: 5.0, 6.0
- Parallelizable: No (foundation for pipelines)

## Implementation Details
Use `github.com/tmc/langchaingo` as the underlying client; hide provider-specifics behind a common interface.

### Relevant Files
- `engine/knowledge/embedder/*`

### Dependent Files
- `engine/knowledge/ingest/*`
- `engine/knowledge/retriever/*`

## Success Criteria
- Adapters compile; unit tests pass; provider errors are descriptive.
