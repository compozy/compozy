# Docs Plan: Knowledge Bases, Embeddings, and Retrieval (MVP)

Source of truth: tasks/prd-embedding/\_prd.md and tasks/prd-embedding/\_techspec.md (see local files for full details). This document lists the documentation work required to land the feature across the docs site, with precise file paths, page outlines, and cross-link updates. No assumptions beyond PRD/TechSpec.

## Goals

- Add first-class docs for declaring and using Knowledge Bases (embedders + vector DBs + knowledge definitions) in YAML.
- Document manual ingestion flows, retrieval/injection behavior, API and minimal CLI surfaces, and observability.
- Update schemas and navigation so authors can find configuration quickly and copy working snippets.

## New Pages (Core Guides)

- docs/content/docs/core/knowledge/overview.mdx (new)
  - Purpose: Conceptual overview, personas, when to use knowledge vs. memory; glossary (embedder, vector DB, knowledge base, source, chunk, retrieval, filters).
  - Outline:
    - Overview and concepts
    - How it fits runtime (resolution → retrieval → prompt injection)
    - Scope and precedence (project → workflow → task → agent)
    - Safety and token budgeting
    - Links: ingestion, retrieval, schema

- docs/content/docs/core/knowledge/configuration.mdx (new)
  - Purpose: YAML configuration for embedders, vector DBs, knowledge bases, bindings; inheritance examples.
  - Outline:
    - Declaring embedders (provider, model, batching)
    - Declaring vector DBs (type, DSN/host, collections)
    - Declaring knowledge bases (sources, chunking, retrieval defaults)
    - Binding knowledge in workflow/task/agent
    - Autoload/import/export behavior

- docs/content/docs/core/knowledge/ingestion.mdx (new)
  - Purpose: Sources and ingestion flows (CLI/API), manual only.
  - Outline:
    - Supported source kinds: pdf_url, markdown_glob (cloud_storage/media optional if implemented)
    - Chunking policies (size/overlap/strategy)
    - Ingestion via CLI/API; idempotency via hashes/ETags
    - Re‑ingest to pick up changes; no background sync in MVP
    - Tip: For freshness, operators can script periodic CLI ingests via cron until background sync is available post‑MVP

- docs/content/docs/core/knowledge/retrieval-injection.mdx (new)
  - Purpose: Retrieval parameters and deterministic prompt injection.
  - Outline:
    - Retrieval mode: dense only in MVP
    - Parameters: top_k, min_score; optional simple tag filters (exact match)
    - Returned payload shape: inline text only
    - Runtime injection order and safeguards

- docs/content/docs/core/knowledge/observability.mdx (new)
  - Purpose: Metrics, logs, tracing, dashboards.
  - Outline:
    - Metrics: knowledge_ingest_duration_seconds, knowledge_chunks_total, knowledge_query_latency_seconds, knowledge_ingest_failures_total
    - Structured logs (ingestion start/finish, failures) via logger.FromContext(ctx)
    - Tracing spans around providers and vector queries
    - Alerts and thresholds

## Schema Docs

- docs/content/docs/schema/embedders.mdx (new)
  - Renders `schemas/embedder.json` once added.
  - Covers provider, model, batching, API key env, timeouts.

- docs/content/docs/schema/vector-dbs.mdx (new)
  - Renders `schemas/vectordb.json` once added.
  - Initial release scope: pgvector and qdrant (examples + tests). Note: Weaviate and LanceDB are planned; document as “coming soon” to avoid over‑promising.

- docs/content/docs/schema/knowledge-bases.mdx (new)
  - Renders `schemas/knowledge-base.json` once added.
  - Covers sources, chunking, retrieval defaults, tags.

- docs/content/docs/schema/bindings.mdx (new)
  - Renders `schemas/knowledge-binding.json` once added.
  - Documents how workflows/tasks/agents reference knowledge by ID and override retrieval params.

- Update: docs/content/docs/schema/project.mdx (update)
  - Include new schema refs once JSON Schemas are added: embedders, vector_dbs, knowledge_bases.
  - Add “Resources” links to new pages.

- Update: docs/content/docs/schema/workflows.mdx (update)
  - Add “Knowledge Binding” section showing binding schema and examples.

## API Docs

- docs/content/docs/api/knowledge.mdx (new)
  - Purpose: API overview for knowledge surfaces with links to endpoints.
  - Outline:
    - Resource model and ETag semantics
    - CRUD: /knowledge-bases (list/get/apply/delete)
    - Actions:
      - POST /knowledge-bases/{id}/ingest
      - POST /knowledge-bases/{id}/query
    - Example requests/responses (curl + JSON), errors (problem+json)
  - Implementation notes:
    - Ensure `swagger.yaml` and `swagger.json` include these endpoints; docs/scripts already load swagger.

## CLI Docs

- docs/content/docs/cli/knowledge-commands.mdx (new)
  - Command group: `compozy knowledge`
  - Subcommands: list, get, apply, delete, ingest, query
  - Flags: `--project`, `--output json`, pagination
  - Clarify semantics: `ingest` is idempotent and can be re‑run to refresh content.
  - Walkthroughs tied to each example (see \_examples.md)
  - Add entry to docs/content/docs/cli/meta.json for navigation

## Cross‑page Updates

- docs/content/docs/core/agents/llm-integration.mdx (update)
  - Add “Knowledge Retrieval” section (where injection occurs, ordering with tools/memory, token budgeting).

- docs/content/docs/core/configuration/global.mdx (update)
  - Reference global feature toggles for knowledge (if any) via `config.FromContext(ctx)`.

- docs/content/docs/api/overview.mdx (update)
  - Link to `api/knowledge` and add a short example for query/ingest.

## Binding Cardinality (MVP)

- MVP supports a single knowledge binding per workflow (simplifies ranking/merging and token budgeting). Document as a single object (`knowledge: { id: ... }`).
- Multi‑KB bindings are “planned”; when implemented, add a dedicated section covering ranking/merging, de‑duplication, and token budget allocation across KBs.

## JSON Schemas to Add (repo root `schemas/`)

- schemas/embedder.json (new)
- schemas/vectordb.json (new)
- schemas/knowledge-base.json (new)
- schemas/knowledge-binding.json (new)
- Update: schemas/compozy.json (add arrays: `embedders`, `vector_dbs`, `knowledge_bases`)
- Update: schemas/workflow.json / task/agent schemas (add binding fields)

Docs pages above must import and render these once available. Keep “Resources” section consistent with other schema pages.

## Navigation & Indexing

- Update docs/source.config.ts to register a new “Knowledge” group under Core.
- Ensure sidebar ordering: Overview → Configuration → Ingestion → Retrieval/Injection → Observability.

## Copy/Style Considerations

- Use existing frontmatter with title/description/icon; follow tone of core docs.
- Provide copy‑pasta YAML with comments matching schema keys from TechSpec (no placeholders that are not in PRD).
- Link to examples under each section.
- Add security callouts where relevant (e.g., prefer SSL for production DSNs; never commit secrets; use env interpolation).
- For observability docs, focus on user‑visible fields and metrics names (avoid internal function names like `logger.FromContext` in the final user copy).

## Acceptance Criteria

- All listed pages created/updated with proposed outlines and cross‑links.
- Swagger includes knowledge endpoints and renders without errors in the API Overview and Knowledge page.
- Internal links validated by docs build (`make dev` in docs folder renders without missing routes).
- Examples referenced exist (per \_examples.md) and are runnable.
- No backwards‑compatibility promise text; clearly marked as new feature.

## Out‑of‑Scope (Doc phase)

- Provider‑specific tuning and hybrid retrieval nuances unless implemented per TechSpec.
- UI guides (no UI required for MVP).

## Open Questions (to confirm during implementation)

- Final endpoint paths in swagger.
- Hybrid retrieval is out of scope for MVP; mark as “planned”.
- Exact environment variables for provider credentials (mirror provider docs).
- Confirmation of an optional `in_memory` vector DB for zero‑dep quickstart. If included, document under `vector-dbs.mdx` and make it the default in the Quickstart example.
