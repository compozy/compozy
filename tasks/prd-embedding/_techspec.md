# Technical Specification (MVP Scope)

## Executive Summary

We will introduce first‑class knowledge base support that pairs reusable embedders, pluggable vector databases, and declarative knowledge base definitions directly inside Compozy project YAML. The engine adds a focused `engine/knowledge` domain for ingestion (chunk → embed → persist) and retrieval (dense similarity only). We explicitly avoid over‑engineering in the MVP: no dedicated ingestion worker binary, no background schedules, no external blob stores, no hybrid retrieval, and no advanced filter DSL. Provider integrations rely on LangChainGo’s embeddings/vector store abstractions (`EmbedDocuments`, `EmbedQuery`) to wrap OpenAI, Vertex AI, and self‑hosted models through a shared interface without bespoke SDK glue.¹

The spec also introduces REST APIs and CLI flows for CRUD, ingestion, and ad‑hoc querying, matching Compozy’s existing resource ergonomics. Developers will be able to declare sources (PDF URLs, local patterns, optional cloud listings), chunk policies, trigger ingestion via CLI/API, and consume retrieval results inside agents and tasks through resolved context injections.

## System Architecture

### Domain Placement

- `engine/knowledge/` (new): domain-layer knowledge base orchestration including config validation, service interfaces, chunkers, ingestion jobs, and retrieval adapters (subpackages `embedder`, `vectordb`, `ingest`, `retriever`).
- `engine/project/`: extend project config with `Embedders`, `VectorDBs`, and `KnowledgeBases` collections plus validation/normalization.
- `engine/workflow/`, `engine/task/`, `engine/agent/`: add inheritance-aware knowledge bindings so workflows, tasks, and agents can opt into default knowledge bases and retrieval parameters without duplicating config.
- `engine/llm/`: enrich the service/orchestrator to resolve knowledge bindings, fetch retrieved passages, and merge them into prompts pre-call.
- `engine/resources/`: add `ResourceKnowledgeBase`, `ResourceEmbedder`, and `ResourceVectorDB` for autoload/export/import.
- `engine/infra/server/router`: register `/knowledge-bases` APIs plus ingestion/query handlers.
- `cli/cmd/knowledge/`: new resource CLI (import/export/ingest/query) leveraging the shared resource command helpers.
- `pkg/config`: expose knowledge settings via `config.FromContext(ctx)` for runtime feature toggles.

### Component Overview

- **Knowledge Config Layer** (`engine/knowledge/config.go`): defines embedders, vector DBs, and knowledge base structs; performs validation (IDs, provider availability, chunk rules, secrets interpolation).
- **Embedder & Vector Providers** (`engine/knowledge/embedder`, `engine/knowledge/vectordb`): wrap LangChainGo embedder/vector-store clients so we inherit batching, similarity search, and provider catalogue.¹
- **Ingestion Pipeline** (`engine/knowledge/ingest`): handles source enumeration, hashing/idempotency, chunking, deduplication, and write batching (pgvector, qdrant, in‑memory). Implemented as a simple 3‑step flow (enumerate → embed+persist batches → finalize) executed within existing worker infrastructure; no dedicated binary.
- **Retrieval Service** (`engine/knowledge/service.go`): resolves knowledge bindings from project/workflow/task/agent context and executes dense similarity searches; merges metadata with workflow context. Hybrid retrieval and advanced filter DSL are deferred.
- **API & CLI Surface**: HTTP endpoints with strong ETag semantics plus CLI wrappers using existing resource import/export helpers.

#### Context Map

- **Project YAML** → `engine/project.Config` → registers embedders/vector DBs/knowledge bases in the resource store.
- **Workflow/Task/Agent YAML** → knowledge binding structs (`core.KnowledgeBinding`) → `engine/knowledge/service` resolution.
- **CLI/HTTP** → `engine/infra/server/router/knowledge` → ingestion/query use cases.
- **Runtime Execution** (`engine/llm/service`) ← knowledge retrieval results ← `engine/knowledge/retriever` ← vector DB providers.

#### Dependency & Flow Map

1. YAML load indexes embedders/vector DBs/knowledge bases → resource store.
2. When a workflow/agent runs, `engine/llm/service` asks `engine/knowledge/service` for resolved bindings → obtains embedder + vector store handles.
3. Retrieval pipeline fetches top chunks (with metadata/tags) and injects them into prompt assembly before LLM invocation.
4. CLI/API ingestion triggers `engine/knowledge/ingest` which streams sources, chunks via configured policies, generates embeddings through selected provider, and persists vectors plus metadata.
5. Monitoring emits metrics/logs via `logger.FromContext(ctx)` and `config.FromContext(ctx)` for rate limits and feature toggles.

## Implementation Design

### Core Interfaces

```go
// Embedder abstracts provider-specific embedding generation.
type Embedder interface {
    EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)
    EmbedQuery(ctx context.Context, text string) ([]float32, error)
}

// VectorStore defines minimal retrieval contract used by knowledge bases.
type VectorStore interface {
    Upsert(ctx context.Context, records []Record) error
    SimilaritySearch(ctx context.Context, query []float32, opts SearchOptions) ([]DocumentMatch, error)
    DeleteByFilter(ctx context.Context, filter Filter) error
}

type Service interface {
    Ingest(ctx context.Context, kbID string, req IngestRequest) (jobID string, err error)
    Query(ctx context.Context, kbID string, req QueryRequest) (*QueryResponse, error)
    ResolveBindings(ctx context.Context, scope Scope) ([]ResolvedBinding, error)
}
```

Interfaces wrap LangChainGo implementations to keep provider churn low (OpenAI, Vertex AI, Cybertron/Hugging Face) while allowing custom injectors.

MVP retrieval returns matches with inline text only (no external blob references):

```go
type DocumentMatch struct {
    ID       string
    Score    float64
    Metadata map[string]any // tags, source, lineage
    Text     string         // chunk text stored inline in vector metadata
}
```

### Data Models

- **EmbedderConfig**: `{ id, provider, model, api_key, config{dimension, batch_size, strip_newlines, retry} }`. Validation ensures `config.dimension` is present when required by the provider and matches the referenced vector database dimension.
- **VectorDBConfig**: `{ id, type (pgvector|qdrant|memory), config{dsn, table, index, metric, dimension, consistency, auth} }`. During project load we cross‑check `config.dimension` against the selected embedder and fail fast on mismatches. Support for Weaviate/LanceDB is planned post‑MVP.
- **KnowledgeBaseConfig** (MVP): `{ id, description, embedder_ref, vector_db_ref, sources[], chunking{strategy, size, overlap}, preprocess{dedupe, remove_html}, retrieval{top_k, min_score}, metadata{tags, owners} }`.
  - Retrieval is dense only; payloads are stored inline within vector metadata.
- **Supported source types** (MVP):
  - `url`: Fetch and process a small public document (PDF, HTML, Markdown, etc.).
  - `markdown_glob`: Ingest local Markdown files matching a glob pattern.
  - `cloud_storage` (optional): S3 or GCS prefix via existing attachment pipeline.
  - `media_transcript` (optional): Ingest `.vtt`/`.srt` transcript files.
    Additional connectors (e.g., `github_repo`, `google_drive`, `web_url`) are deferred post‑MVP.
- **KnowledgeBinding** (workflow/task/agent): `{ id_ref, top_k, min_score, max_tokens, fallback, inject_as }` supporting overrides at each hierarchy level. Simple tag filters MAY be introduced post‑MVP as a `map[string]string` (exact match).

Chunk defaults follow LangChain RAG guidance (256–1024 token windows, overlap 10–20%) balancing recall and latency.

### Ingestion Flow (MVP)

Executed within existing worker infrastructure as a straightforward 3‑step flow:
(Leverage Compozy's core Temporal workers/queues; no new task queues or binaries are introduced in MVP.)

1. `EnumerateAndChunk` – resolve sources via attachment resolvers, expand globs, and emit chunk batches with stable IDs;
2. `EmbedAndPersistBatch` – batch call embedder, then upsert vectors + metadata into the configured store;
3. `Finalize` – record counts and emit metrics via `logger.FromContext(ctx)`.
   MVP Ingestion Activities (executed in existing worker):
4. `EnumerateAndChunk` – discover sources and produce chunks with stable IDs (hash of kb_id + path + content);
5. `EmbedAndPersistBatch` – batch embed chunks and write vectors/metadata to the configured store with idempotent upserts;
6. `Finalize` – record counts and emit metrics.

There is no dedicated ingestion worker binary, no custom schedules, and no job‑polling surface in MVP.

#### Storage Lifecycle

- MVP stores chunk payloads inline in vector DB metadata (bounded by conservative size; external blob stores are out of scope).
- Delete removes vector rows only. No external blob purge in MVP.

### High-Level YAML APIs

**Project Level** – declare embedders, vector DBs, and knowledge bases:

```yaml
embedders:
  - id: openai_embedder
    provider: openai
    model: text-embedding-3-small
    api_key: "{{ .env.OPENAI_API_KEY }}"
    config:
      batch_size: 96

vector_dbs:
  - id: pgvector_main
    type: pgvector
    config:
      dsn: "{{ .secrets.PGVECTOR_DSN }}"
      table: knowledge_chunks
      ensure_index: true

knowledge_bases:
  - id: support_docs
    description: Customer support handbook
    embedder: openai_embedder
    vector_db: pgvector_main
    sources:
      - type: url
        urls:
          - "https://example.com/support.pdf"
      - type: markdown_glob
        path: "docs/support/**/*.md"
    chunking:
      strategy: recursive_text_splitter
      size: 800
      overlap: 120
    retrieval:
      top_k: 8
      min_score: 0.1
```

**Workflow Level** – declare workflow-scoped knowledge bases directly inside the workflow YAML (same schema, but scoped to that workflow only):

```yaml
# workflows/support-assistant.yaml
id: support-assistant
version: 0.1.0

knowledge_bases:
  - id: wf_support_docs
    description: "Support playbook snippets"
    embedder: openai_embedder # references project-level embedder
    vector_db: pgvector_main # references project-level vector DB
    sources:
      - type: markdown_glob
        path: "./workflows/support/playbooks/*.md"
      - type: media_transcript
        provider: youtube
        video_id: "{{ .workflow.input.training_video_id }}"
    retrieval:
      top_k: 4
      inject_as: "{{ .workflow.vars.support_docs }}"

tasks:
  - id: triage_ticket
    type: basic
    agent: support_agent
    knowledge:
      - id: wf_support_docs
    # ...
```

This mirrors the way other workflow assets (`tools`, `agents`, `tasks`) are defined: `knowledge_bases` becomes a first-class top-level section in workflow configs, and their availability is limited to that workflow. They can still reuse project-defined embedders/vector DBs by reference.

**Task Level** – reference knowledge explicitly for fine-tuned retrieval:

```yaml
tasks:
  - id: triage_ticket
    type: basic
    agent: support_agent
    knowledge:
      - id: wf_support_docs
        top_k: 3
```

**Agent Level** – declare preferred knowledge bases when the agent runs outside workflow defaults:

```yaml
agents:
  - resource: agent
    id: support_agent
    instructions: |
      You are a support expert. Prefer sourced answers.
    knowledge:
      - id: support_docs
        inject_as: context_docs
        max_tokens: 1500
```

The resolver first loads workflow-scoped knowledge bases (from the workflow YAML), then falls back to project-level definitions, and finally applies task/agent inline bindings. This guarantees deterministic resolution while keeping workflow-private knowledge isolated from the rest of the project.

### API Endpoints (MVP)

| Method & Path                                 | Purpose                                       | Notes                                              |
| --------------------------------------------- | --------------------------------------------- | -------------------------------------------------- |
| `GET /api/v0/knowledge-bases`                 | List knowledge bases (pagination)             | Mirrors workflow listing semantics (limit/cursor). |
| `GET /api/v0/knowledge-bases/{kb_id}`         | Fetch config                                  | Supports `If-None-Match`.                          |
| `PUT /api/v0/knowledge-bases/{kb_id}`         | Create/update knowledge base                  | Requires ETag on update; validates references.     |
| `DELETE /api/v0/knowledge-bases/{kb_id}`      | Remove definition and vectors (vector‑only)   | No `purge` option in MVP.                          |
| `POST /api/v0/knowledge-bases/{kb_id}/ingest` | Trigger ingestion based on configured sources | Accepts `strategy` (upsert, replace).              |
| `POST /api/v0/knowledge-bases/{kb_id}/query`  | Execute retrieval (text query)                | Dense only; optional simple tag filters.           |

Delete performs vector‑only removal. Response envelopes reuse `router.Response` and return ETags.

### CLI Surface (MVP)

New commands live under `compozy knowledge`:

| Command                           | Description                                         | Flags                                 |
| --------------------------------- | --------------------------------------------------- | ------------------------------------- |
| `compozy knowledge list`          | Lists knowledge bases (project‑scoped)              | `--project`                           |
| `compozy knowledge get <id>`      | Shows a single knowledge base                       | `--project`                           |
| `compozy knowledge apply -f <y>`  | Applies YAML definition(s) (create/update)          | `--project`                           |
| `compozy knowledge delete <id>`   | Removes a knowledge base (vector‑only deletion)     | `--project`                           |
| `compozy knowledge ingest <id>`   | Runs ingestion for configured sources               | `--project`, `--strategy`             |
| `compozy knowledge query <id> -q` | Executes dense similarity search and prints matches | `--project`, `--top-k`, `--min-score` |

All commands share auth/project flags provided by `cli/cmd/resource` helpers. No `inspect`, no `--watch`, and no `--expand` in MVP.

## Integration Points

- **Embedding Providers**: wrap LangChainGo embedder clients (OpenAI, Vertex AI, Cybertron/Hugging Face) to avoid custom SDK glue and reuse batching support.¹
- **Vector Stores**: leverage LangChainGo vector store drivers (pgvector, Qdrant, Weaviate, LanceDB) for storage and similarity search semantics, with adapter glue for knowledge service contracts.¹

- **Monitoring**: emit Prometheus metrics via `infra/monitoring` (ingestion latency, chunk counts, query latency, vector size).

### Attachment & Resolver Integration

- **Reuse existing normalization pipeline**: Knowledge bindings MUST call the same two-phase normalization, templating, and precedence logic already implemented for attachments. Workflow/task/agent knowledge definitions are resolved through the template engine and merged via an `attachment.ComputeEffectiveItems`-style helper to preserve deterministic overrides (`engine/task/uc/exec_task.go`, `engine/attachment/merge.go`).
- **Source resolver alignment**: All filesystem/HTTP/blob fetching logic delegates to the attachment resolvers (`engine/attachment/resolver_*`). The knowledge ingest job operates on resolved handles to avoid duplicating path sanitation, CWD scoping, or MIME checks.
- **Shared cleanup semantics**: Conversion to `llmadapter.ContentPart` stays centralized in `attachment.ToContentPartsFromEffective`, ensuring chunk previews or inline snippets follow the same memory lifecycle (including cleanup callbacks) already used by agents today.
- **Deterministic binding precedence**: Resolution walks scopes in order `project → workflow → task → agent`. We treat each knowledge entry as a map keyed by `id_ref`; when the same ID appears in multiple scopes we deep-merge structs, with later scopes overriding scalar values and replacing slices. Duplicate IDs within the same scope are rejected at validation time. The merged bindings are sorted by first appearance to guarantee stable retrieval order.

### LLM Runtime Contract Changes

- **New retrieved context struct**: Introduce `knowledge.RetrievedContext` with fields `BindingID`, `Content`, `Score`, `TokenEstimate`, and optional `Metadata`. This struct is produced by `engine/knowledge/service` after retrieval and before prompt assembly.
- **Service signature update**: Extend `engine/llm/service.GenerateContent` to accept an additional `retrievedContexts []knowledge.RetrievedContext` parameter. The orchestrator request gains a matching field so downstream prompt builders can render context consistently (e.g., prepend citations, enforce token caps).
- **Prompt builder responsibility**: `engine/llm/orchestrator` owns deterministic rendering—agents/actions do not inline raw strings. Builders apply configurable strategies (bullet list, JSON blocks) and honor `max_tokens` or `inject_as` instructions supplied in knowledge bindings.
- **Testing expectations**: Update orchestrator and task execution tests to assert ordering, deduplication, and truncation of retrieved context, mirroring existing attachment coverage. Mocks assert that `GenerateContent` receives both attachments and retrieved contexts during execution.
- **Token estimation rule**: `knowledge.RetrievedContext.TokenEstimate` is computed during retrieval using provider-specific tokenizers when available (e.g., tiktoken for OpenAI) and a `len(rune)/4` fallback. Prompt builders enforce the `max_tokens` guard by trimming lowest scoring chunks before calling the LLM.

### Resource, Autoload, and CLI Extensions

- **New config types**: Add `core.ConfigKnowledgeBase`, `core.ConfigEmbedder`, and `core.ConfigVectorDB` in `engine/core/config.go`, propagate them through `engine/resources/store.go`, and register them with the autoload registry (`engine/autoload/registry.go`) so indexing, provenance metadata, and import/export flows work out of the box.
- **Resource store integration**: Extend `engine/resources/meta.go` helpers and any switch statements that enumerate resource types to persist ETag/metadata for knowledge assets, ensuring CLI diffing tooling remains consistent.
- **CLI/HTTP parity**: Implement `cli/cmd/knowledge` atop the shared resource helpers (`cli/cmd/resource`) and ensure new REST routes live under `engine/infra/server/router/knowledge`. Commands must support auth/project scoping and ETag enforcement.
- **Backward-compat guardrails**: Because the project intentionally avoids backwards compatibility guarantees, the migration guide must call out breaking YAML schema changes, but the autoloader should emit explicit errors if legacy configs reference missing knowledge resources to aid adoption.

## Impact Analysis

| Affected Component                   | Impact Type        | Description & Risk Level                                                                                          | Required Action                                     | Priority |
| ------------------------------------ | ------------------ | ----------------------------------------------------------------------------------------------------------------- | --------------------------------------------------- | -------- |
| `engine/project.Config`              | Schema Extension   | Add embedders/vector_dbs/knowledge_bases arrays with validation and migration guard. Risk: medium (config churn). | Implement struct & validation, add migration notes. | P0       |
| `engine/workflow/task/agent` configs | Config Inheritance | Introduce knowledge bindings with hierarchical merge and schema updates. Risk: medium.                            | Update structs, mapstructure tags, docs/tests.      | P0       |
| `engine/llm/service`                 | Runtime Behavior   | Inject retrieval context before LLM calls; ensure backwards compatibility. Risk: medium.                          | Add resolver hooks with feature flag fallback.      | P0       |
| `engine/resources`                   | Resource Indexing  | Register new resource types and importer/exporter support. Risk: low.                                             | Extend enums, autoload, metadata writes.            | P1       |
| API Router & CLI                     | New Surface        | Add routes & CLI commands; ensure auth and rate limits. Risk: low.                                                | Implement endpoints, swagger docs, CLI wrappers.    | P1       |

## Testing Approach

### Unit Tests

- `engine/knowledge/config_test.go`: validation (missing references, invalid chunk sizes, defaults).
- `engine/knowledge/ingest_test.go`: chunking correctness, batching, dedupe, hash-based idempotency.
- `engine/knowledge/retriever_test.go`: retrieval returns ordered results; respects `top_k`, simple tag filters, and trims by `max_tokens`.
- `engine/knowledge/service_test.go`: binding resolution across project/workflow/task/agent scopes.
- `engine/project/config_test.go`: ensure knowledge definitions survive mapstructure decode.
- Router tests for API contract (ETag, errors). No job polling routes.

### Integration Tests

- `test/integration/knowledge/pgvector_test.go`: spin up pgvector (Docker) and verify ingestion/query flows.
- `test/integration/knowledge/workflow_binding_test.go`: run sample workflow referencing knowledge base to ensure retrieval context flows into LLM stub.
- `test/integration/knowledge/cli_test.go`: validate `compozy knowledge` commands against mocked server.

## Development Sequencing

### Build Order

1. **Config & Resource Types** – define structs, resource enums, schema validation.
2. **Knowledge Domain Package** – implement embedder/vector adapters, ingestion, retrieval service.
3. **Runtime Integration** – wire knowledge service into `engine/llm/service` and task/agent resolvers; add prompt injection logic.
4. **API & CLI** – expose management endpoints and commands (no job status polling).
5. **Docs & Samples** – update guides and provide reference YAML and CLI walkthroughs.

### Technical Dependencies

- pgvector or Qdrant test containers for integration tests.
- LangChainGo dependency bump (ensure version exposing embedder/vector interfaces we rely on).
- API documentation updates (OpenAPI, CLI help).

## Monitoring & Observability

- Metrics: `knowledge_ingest_duration_seconds`, `knowledge_chunks_total`, `knowledge_query_latency_seconds` (label with `kb_id`).
- Structured logs: ingestion start/finish, chunk counts, failure reasons (using `logger.FromContext(ctx)`).
- Tracing: wrap embedder + vector store calls with OTel spans; include provider/model metadata.
- Alerts: high failure rate or latency via existing Grafana dashboards.

## Technical Considerations

### Key Decisions

- **Provider Abstraction**: Use LangChainGo embeddings/vector stores to avoid bespoke SDK layers while supporting multiple providers (OpenAI, Vertex AI, local).
- **Knowledge Base Shape**: Keep configuration explicit and minimal: embedder + vector DB + sources + chunking + retrieval (dense only).
- **Chunking Defaults**: Follow LangChain guidance (chunk size 256–1024 tokens, overlap 10–20%) and allow overrides per knowledge base to balance recall and speed.
- **DX Priority**: Provide autoload support and minimal CLI parity. Defer additional DX tooling (e.g., `inspect`).

### Known Risks

- Large document ingestion may stress memory or exceed provider rate limits—mitigate with configurable batch sizes and backpressure.
- Vector schema migrations (pgvector index changes) require careful rollout; include `ensure_schema` opt-out.
- Retrieval latency could impact LLM turn time; support caching and min_score thresholds to skip low-signal matches.

### Special Requirements

- All runtime code must derive logger/config via context (`logger.FromContext`, `config.FromContext`); forbid globals.
- Knowledge bindings must respect existing inheritance rules (project → workflow → task → agent) without breaking current configs.
- Secrets (API keys, DSNs) must be referenced via env/secrets interpolation—no plaintext secrets in YAML.
- Ensure no backwards compatibility guarantees per project policy, but offer migration guidance.

### Risk & Assumptions Registry

| Risk / Assumption                                                          | Mitigation / Follow-up                                                         | Owner          |
| -------------------------------------------------------------------------- | ------------------------------------------------------------------------------ | -------------- |
| Assumes pgvector or alternative vector DB available in target deployments. | Provide in-memory fallback for dev/testing and document setup.                 | Infra          |
| Potential rate limiting from embedder providers during bulk ingest.        | Add adaptive batching/retry with jitter and expose knobs in config.            | Knowledge Team |
| Users may require filtered retrieval (tags, metadata).                     | Implement filter schema in config and enforce via vector store query builders. | Knowledge Team |
| Need for reranking beyond vector similarity.                               | Expose optional reranker provider hook and default to similarity-only.         | LLM Team       |

### Libraries Assessment

| Candidate                                          | License / Health                                                          | Pros                                                                         | Cons                                                             | Recommendation                                                   |
| -------------------------------------------------- | ------------------------------------------------------------------------- | ---------------------------------------------------------------------------- | ---------------------------------------------------------------- | ---------------------------------------------------------------- |
| LangChainGo (github.com/langchain-ai/langchain-go) | MIT, actively maintained, multi-provider embeddings/vector integrations.¹ | Rich embedder/vector adapters, batching utilities, aligns with Go ecosystem. | Requires dependency updates and adapter glue for our interfaces. | Adopt as primary abstraction layer.                              |
| Direct OpenAI/Vertex SDKs                          | Vendor-specific                                                           | Minimal surface, direct control.                                             | No multi-provider abstraction, duplicate code per provider.      | Use only via LangChainGo embedder clients to reduce duplication. |
| Custom in-house adapters                           | N/A                                                                       | Tailored APIs.                                                               | High maintenance, slower provider support.                       | Avoid—prefer LangChainGo.                                        |

### Standards Compliance

| Standard                                | Plan for Compliance                                                                                       |
| --------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| `.cursor/rules/architecture.mdc`        | New `engine/knowledge` follows clean architecture: domain services + adapters; dependencies flow inward.  |
| `.cursor/rules/go-coding-standards.mdc` | All new Go code keeps functions under limits, returns contextual errors, avoids blank lines in functions. |
| `.cursor/rules/test-standards.mdc`      | Unit tests use `t.Run("Should...")`, testify assertions, cover success/failure paths.                     |
| `.cursor/rules/api-standards.mdc`       | API endpoints use versioned paths, ETag, pagination, problem+json errors.                                 |
| `.cursor/rules/magic-numbers.mdc`       | Expose chunk size, batch limits via config constants (no inline literals).                                |

---

¹ LangChainGo embeddings and vector store interfaces: https://pkg.go.dev/github.com/caxqueiroz/langchaingo/embeddings
