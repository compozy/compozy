# Builder Outline: Knowledge System (5 Builders)
- **Purpose:** Explain `sdk/knowledge` builders for embeddings, vector DBs, sources, bases, and bindings. Source: tasks/prd-sdk/03-sdk-entities.md §"Knowledge System".
- **Audience:** Teams implementing retrieval augmented workflows.
- **Sections:**
  1. Component summary table (Embedder, VectorDB, Source, Base, Binding) referencing method list.
  2. Data ingestion flows referencing 04-implementation-plan.md knowledge ingestion tasks.
  3. Security & governance (PII handling, access controls) referencing _techspec.md security guardrails.
  4. Operational considerations (sync jobs, monitoring) referencing 02-architecture.md knowledge lifecycle.
  5. Validation/testing guidance referencing 07-testing-strategy.md knowledge fixtures.
- **Content Sources:** 03-sdk-entities.md, 04-implementation-plan.md, 05-examples.md knowledge examples.
- **Cross-links:** Core knowledge base doc, memory builder page, troubleshooting for ingestion errors.
- **Examples:** `sdk/examples/05_knowledge_search.go`, `06_multi_agent_pipeline.go`.
