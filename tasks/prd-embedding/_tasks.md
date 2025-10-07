# [prd-embedding] Implementation Task Summary

## Relevant Files

### Core Implementation Files

- `engine/knowledge/config.go` - Knowledge config types and validation
- `engine/knowledge/embedder/` - Embedder adapters wrapping LangChainGo providers
- `engine/knowledge/vectordb/` - Vector DB adapters (pgvector, qdrant, memory)
- `engine/knowledge/chunk/` - Chunking strategies, preprocess, hashing
- `engine/knowledge/ingest/` - Ingestion pipeline (enumerate → embed → persist)
- `engine/knowledge/retriever/` - Dense similarity retrieval and scoring
- `engine/knowledge/service.go` - Binding resolution + retrieval façade
- `engine/project/` - Project/workflow/task/agent binding extensions
- `engine/llm/orchestrator/` - Prompt injection integration
- `engine/infra/server/router/knowledge/` - HTTP CRUD, ingest, query endpoints
- `cli/cmd/knowledge/` - CLI group and subcommands
- `pkg/config/` - Feature toggles via `config.FromContext(ctx)`

### Schemas

- `schemas/embedder.json`
- `schemas/vectordb.json`
- `schemas/knowledge-base.json`
- `schemas/knowledge-binding.json`
- Updates: `schemas/project.json`, `schemas/workflow.json`

### Documentation Files

- `docs/content/docs/core/knowledge/overview.mdx`
- `docs/content/docs/core/knowledge/configuration.mdx`
- `docs/content/docs/core/knowledge/ingestion.mdx`
- `docs/content/docs/core/knowledge/retrieval-injection.mdx`
- `docs/content/docs/core/knowledge/observability.mdx`
- `docs/content/docs/schema/{embedders,vector-dbs,knowledge-bases,bindings}.mdx`
- `docs/content/docs/api/knowledge.mdx`
- `docs/content/docs/cli/knowledge-commands.mdx`

### Examples

- `examples/knowledge/quickstart-markdown-glob/*`
- `examples/knowledge/pgvector-basic/*`
- `examples/knowledge/pdf-url/*`
- `examples/knowledge/query-cli/*`

## Tasks

- [x] 0.0 Pre‑work: External Libs Verification + Test Utilities Baseline (S)
- [x] 1.0 Knowledge Domain Scaffolding (M)
- [x] 2.0 Embedder Adapters (M)
- [x] 3.0 Vector DB Adapters (M)
- [x] 4.0 Chunking & Preprocess (M)
- [x] 5.0 Ingestion Pipeline (L)
- [x] 6.0 Retrieval Service (M)
- [x] 7.0 YAML & Binding Resolution (M)
- [x] 8.0 JSON Schemas (S)
- [ ] 9.0 LLM Orchestrator Integration (M)
- [ ] 10.0 HTTP APIs (M)
- [ ] 11.0 CLI Commands (S)
- [ ] 12.0 Observability (S)
- [ ] 13.0 Docs Implementation (M)
- [ ] 14.0 Examples Implementation (S)
- [ ] 15.0 Integration & E2E Tests (L)

## Execution Plan

- Critical Path: 1.0 → 2.0 → 3.0 → 5.0 → 6.0 → 9.0 → 10.0 → 11.0 → 13.0 → 15.0
- Parallel Track A (after 1.0): 4.0 → 5.0
- Parallel Track B (after 1.0): 7.0 → 8.0
- Parallel Track C (after 3.0): 12.0

Notes

- Each task contains its own unit‑test subtasks; integration/E2E consolidate under 15.0.
- All runtime code MUST use `logger.FromContext(ctx)` and `config.FromContext(ctx)`; never globals.
- Run `make fmt && make lint && make test` before marking any task as completed.

## Batch Plan (Grouped Commits)

- [x] Batch 1 — Bootstrap (foundation + schemas): 0.0, 1.0, 7.0, 8.0
- [x] Batch 2 — Adapters & Chunking: 2.0, 3.0, 4.0
- [x] Batch 3 — Pipelines: 5.0, 6.0
- [ ] Batch 4 — Runtime: 9.0
- [ ] Batch 5 — Public Surfaces: 10.0, 11.0
- [ ] Batch 6 — Observability: 12.0
- [ ] Batch 7 — Docs + Examples: 13.0, 14.0
- [ ] Batch 8 — Integration/E2E: 15.0
