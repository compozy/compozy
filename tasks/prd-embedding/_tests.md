# Tests Plan: Knowledge Bases, Embeddings, and Retrieval (MVP)

Source of truth for all test coverage required to land the PRD in `tasks/prd-embedding/_prd.md` and Tech Spec in `tasks/prd-embedding/_techspec.md`. This plan enumerates test scopes, concrete files to add, fixtures, and acceptance mappings. All tests must follow `.cursor/rules/test-standards.mdc` and project rules (context, logger, config, magic numbers, API standards).

## Guiding Principles

- Follow t.Run naming: "Should …" for every subtest.
- Use `stretchr/testify` (`require` for fatal, `assert` for specifics). No suites.
- Mock only external dependencies (embedders, vector stores). Prefer pure functions for internal logic.
- Derive logger/config from context in runtime code; test code may use helpers to seed context. Never use `context.Background()` in runtime paths.
- Validate ETag, pagination, error envelopes per API standards.
- Prefer deterministic fixtures and small, local assets; avoid network unless explicitly required by an integration.

## Coverage Matrix (PRD → Tests)

- AC1 Project/Workflow YAML validation → unit tests in `engine/knowledge/config_test.go`, `engine/project/config_test.go`.
- AC2 Binding resolution & precedence (workflow → project → inline) → unit tests in `engine/knowledge/service_test.go` and integration in `test/integration/knowledge/workflow_binding_test.go`.
- AC3 Ingestion via CLI/API (manual) → router/unit in `engine/infra/server/router/knowledge_router_test.go`; integration in `test/integration/knowledge/pgvector_test.go`.
- AC4 Query returns ordered `top_k`, `min_score` → unit in `engine/knowledge/retriever_test.go`; integration with pgvector in `test/integration/knowledge/pgvector_test.go`.
- AC6 Observability (metrics/logs/traces presence) → unit assertions on counters/spans and structured logs in `engine/knowledge/ingest_test.go`, `engine/knowledge/retriever_test.go`.
- AC7 Docs/API parity sanity → router contract tests in `engine/infra/server/router/knowledge_router_test.go` ensure swagger tags/paths exist once generated.

## Unit Tests (Go)

- engine/knowledge/config_test.go:1
  - Should validate missing embedder/vector refs with specific error messages.
  - Should reject invalid chunking (size, overlap, strategy).
  - Should normalize defaults (chunk size/overlap, retrieval `top_k`, `min_score`).
  - Should interpolate env/secret placeholders but reject plaintext secrets.
  - Should reject unsupported `sources[].type` values with specific validation errors.

- engine/knowledge/ingest_test.go:1
  - Should chunk text by configured strategy/size/overlap and produce stable IDs.
  - Should deduplicate by content hash and ensure idempotent re-ingest.
  - Should batch embeddings respecting provider batch size; propagate provider errors with context.
  - Should persist inline payloads in vector metadata.

- remove: storage blob tests (no external blob store in MVP).

- engine/knowledge/retriever_test.go:1
  - Should return ordered results with stable tie-breaking, respect `top_k` and `min_score`.
  - Should trim by `max_tokens` for prompt injection budget.

- remove: purge tests (vector-only deletion in MVP).

- engine/knowledge/service_test.go:1
  - Should resolve bindings with precedence: workflow → project → inline overrides.
  - Should render `inject_as` and default injection location for runtime.

- engine/project/config_test.go:1
  - Should decode `embedders`, `vector_dbs`, `knowledge_bases` through mapstructure without loss.

- engine/infra/server/router/knowledge_router_test.go:1
  - Should enforce ETag on `PUT /knowledge-bases/{id}` updates.
  - Should return `304 Not Modified` on `GET /knowledge-bases/{id}` when `If-None-Match` matches.
  - Should paginate `GET /knowledge-bases` with `limit`/`cursor`.
  - Should validate query body (top_k/min_score) and return problem+json errors.
  - Should allow forward pagination then backward pagination to first page, returning the same item set.

## Integration Tests

- Note: Container-backed tests are opt-in for local runs. Gate with `KNOWLEDGE_E2E=1` (skip when unset) and/or a build tag such as `//go:build knowledge_e2e`. Ensure CI enables these explicitly.

- test/integration/knowledge/pgvector_test.go:1
  - Start pgvector via `testcontainers-go` (image `pgvector/pgvector:pg16`).
  - Apply schema/index if `ensure_index` is set; clean up between runs.
  - Ingest markdown fixtures; assert idempotency and row counts.
  - Query dense retrieval; assert `top_k`, `min_score`, and median latency in logs.
  - Re‑run ingestion for the same KB; assert idempotency (no duplicate rows).

- test/integration/knowledge/qdrant_test.go:1 (P1)
  - Start `qdrant/qdrant` container; provision collection.
  - Ingest sample chunks; query nearest neighbors; validate filters.

- test/integration/knowledge/workflow_binding_test.go:1
  - Load a tiny workflow referencing a workflow-scoped KB; run a stubbed LLM path; assert injected context ordering.

- test/integration/knowledge/cli_test.go:1
  - Exercise `compozy knowledge list/get/apply/delete/ingest/query` against mocked server.
  - Snapshot JSON output for stable flags.

## Fixtures & Testdata

- engine/knowledge/testdata/markdown/\*.md:1
  - 2–3 small files with obvious answers and tags (e.g., `locale`, `product`).

- engine/knowledge/testdata/pdf/sample.pdf:1 (P1)
  - Tiny public-domain PDF mirrored locally.

- engine/knowledge/testdata/transcripts/\*.vtt:1 (P2)
  - Two tiny transcripts for `media_transcript` source.

- engine/knowledge/testdata/yaml/project.yaml:1
  - Minimal `embedders`, `vector_dbs`, `knowledge_bases` definitions (env placeholders only).

## Mocks & Stubs

- embeddings provider (LangChainGo):
  - Stub interface exposing `EmbedDocuments(ctx, []string)` and `EmbedQuery(ctx, string)`; return deterministic vectors.
  - Reference: LangChainGo embeddings docs show `EmbedDocuments` and provider integrations; use `github.com/tmc/langchaingo` v0.1.13 to align with `go.mod`.

- vector store adapter:
  - In-memory stub for unit tests returning deterministic nearest neighbors using cosine similarity.
  - For pgvector/Qdrant, use real containers only in integration tests.

- remove: blob store stubs (no external blob store in MVP).

- remove: job scheduler mocks (no custom schedules or job polling in MVP).

## API Contract Assertions

- ETag enforcement on updates and conditional GETs.
- Problem+json error bodies with type/title/detail/instance.
- Pagination: `limit` and `cursor` symmetry.
- Status codes: 200/201/202/204 for success paths; 400/404/409/412/422 for validation/conflicts/preconditions.
- Swagger parity: compare generated OpenAPI against golden at `engine/infra/server/router/testdata/swagger/knowledge.json`.

## Observability Assertions

- Metrics: increment and histogram buckets for `knowledge_ingest_duration_seconds`, `knowledge_chunks_total`, `knowledge_query_latency_seconds`; label each with `{kb_id}` as applicable.
- Structured logs: start/finish, counts, failure reasons; include `kb_id`.
- Tracing: presence of spans around embedder/vector operations with provider/model attributes.

## Performance & Limits (Deterministic Checks)

- Enforce batch size/concurrency caps for embeddings to stay within provider rate limits (simulate 429 → backoff).
- Verify retrieval path adds bounded latency overhead in integration logs.
- Token budgeting: enforce `max_tokens` trimming in retriever unit tests.

## CLI Tests (Golden)

- JSON output for `list/get` includes IDs, descriptions, and retrieval defaults.
- remove: `inspect` and `sync` coverage for MVP.
- Prefer golden JSON files under `test/integration/knowledge/testdata/cli/` with `UPDATE_GOLDEN` to refresh.

## Examples of Canonical t.Run Names

- "Should enforce ETag precondition on update"
- "Should return error when top_k is negative"
- "Should reject unsupported source type"

## Non‑Goals

- Advanced rerankers beyond optional toggle in Tech Spec.
- Multi‑KB ranking/merging (mark planned if not in initial implementation).

## Test Utilities

- test/helpers/context.go:1
  - `NewTestContext(t *testing.T)` seeds context with test logger and minimal test config (e.g., toggles enabling knowledge features) via `config.FromContext(ctx)`.

- test/helpers/containers.go:1
  - Helpers to start/stop pgvector and qdrant with retries; returns DSN/URL.

- test/helpers/golden.go:1
  - Load/save golden JSON with `UPDATE_GOLDEN` toggle.

## Exit Criteria

- All unit and integration tests above exist and pass on CI.
- Coverage ≥ 80% for `engine/knowledge` and config paths.
- `make fmt && make lint && make test` green locally and on CI.

## Notes on External Libraries

- LangChainGo embeddings and vector store integrations are relied upon for providers and stores; interfaces expose `EmbedDocuments`/`EmbedQuery` and retriever `GetRelevantDocuments`. Use the repository version in `go.mod` to avoid mismatches.
- Use `github.com/testcontainers/testcontainers-go` to provision pgvector and qdrant in integration tests.

## Open Items to Confirm During Implementation

- Final endpoint paths and swagger tags produced by `engine/infra/server/router` for knowledge.
- Hybrid retrieval availability in first cut; if disabled, mark tests as skipped with clear reason.
- Optional in‑memory vector store for unit tests; otherwise rely solely on stubs.
