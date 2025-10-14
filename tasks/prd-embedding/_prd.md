# Product Requirements Document (PRD): Knowledge Bases, Embeddings, and Retrieval

## Overview

Compozy will add first‑class Knowledge Bases that pair reusable Embedders, pluggable Vector Databases, and declarative Knowledge Base definitions in project and workflow YAML. The feature enables teams to index documents from multiple sources and retrieve relevant context for agents, tasks, and workflows with a single line of YAML. The experience includes REST and CLI for CRUD, ingestion, and ad‑hoc query, with results injected into LLM prompts by the runtime orchestrator.

This PRD captures the WHAT and WHY; implementation details live in the Tech Spec at `tasks/prd-embedding/_techspec.md`. For this first release, we intentionally ship a minimal, focused MVP that avoids over‑engineering. Advanced features (hybrid retrieval, background schedulers, blob stores, deep CLI tooling) are explicitly out of scope and deferred.

## Goals

- Consistent DX to define and use knowledge across projects, workflows, tasks, and agents via YAML.
- Support multiple embedding providers and vector stores through a stable product surface (no per‑provider UX differences).
- Provide simple ingestion flows (CLI/API) and query APIs with clear ETag semantics.
- Make retrieved context available to the LLM pipeline deterministically and safely within token budgets.
- Deliver observability (metrics, logs, tracing) for ingestion and retrieval.

Key metrics to track (product level):

- Adoption: number of projects with ≥1 knowledge base; number of workflows/tasks/agents referencing knowledge.
- Reliability: ingestion job success rate (%), query error rate (%).
- Freshness: median time from source change to availability via re‑ingest.
- Performance: p95 retrieval latency per query (top_k≤10) measured server‑side; track over time without hard targets in PRD.

## User Stories

- As a platform engineer, I declare embedders, vector DBs, and knowledge bases in project YAML so teams share consistent infrastructure.
- As a workflow author, I bind a knowledge base and set retrieval parameters (e.g., `top_k`, filters) without writing code.
- As an agent author, I opt‑in to preferred knowledge and control how retrieved context is injected into prompts.
- As an operator, I trigger ingestion and monitor progress via logs/metrics using CLI or API.
- As a developer, I understand the resolved knowledge bindings for a workflow/task/agent through docs and logs.
- As an end user of an agent, I get sourced answers informed by up‑to‑date documents.

## Core Features

R1. Declarative resources in YAML

- Project‑level arrays for `embedders`, `vector_dbs`, and `knowledge_bases` with validation and normalization.

R2. Scope and precedence

- Knowledge can be defined at project and workflow level; tasks/agents bind via IDs. Resolution applies deterministic precedence (workflow → project, then inline bindings).

R3. Source types and ingestion

- Core: `url`, `markdown_glob`.
- Optional (if time/available): `cloud_storage` (S3/GCS; requires credentials via env), `media_transcript`.
- CLI/API for ad‑hoc ingestion only; background sync/schedules are deferred. No bespoke job polling API in MVP.

R4. Chunking and retrieval policy

- Configurable chunking (strategy, size, overlap).
- Retrieval parameters include `top_k`, `min_score`, and a simple tag filter map. MVP is dense‑only; hybrid retrieval and advanced filter DSL are deferred.

R5. APIs

- CRUD for knowledge bases; endpoints for documents ingestion and query with ETag/pagination conventions. No `/sync` or `/jobs` endpoints in MVP.

R6. CLI

- `compozy knowledge` namespace for list/get/apply/delete/ingest/query with project scoping. No `inspect`, `sync`, `--watch`, or `--expand` in MVP.

R7. Runtime integration

- Retrieved contexts are resolved before LLM invocation and passed through orchestrator builders that render them deterministically.

R8. Observability and governance

- Metrics, structured logs, and tracing for ingestion/retrieval. Delete removes vector rows only; purge/cascade deletes are deferred (no external blob store support in MVP).

## User Experience

- Personas: platform engineer (sets infra), workflow/agent author (uses knowledge), operator (runs ingestion), end user (benefits from better answers).
- Flows: define resources in project YAML; optionally add workflow‑scoped knowledge; run `compozy knowledge apply` then `compozy knowledge ingest`; bind knowledge in workflow/task/agent; observe logs/metrics; query via CLI/API for testing.
- Accessibility: CLI commands and API responses follow existing conventions; documentation includes copy‑pasta examples; no UI changes required for MVP.

## High‑Level Technical Constraints

- Integrations: embedding providers (e.g., OpenAI, Vertex, local) and vector stores (e.g., pgvector, Qdrant) behind a unified product surface.
- Compliance/security: secrets provided via env/secret interpolation in YAML; no plaintext secrets.
- Performance/scalability: retrieval adds low latency overhead relative to LLM calls; ingestion scales via batching and backpressure; targets are tracked as metrics (final thresholds set in Tech Spec rollout).
- Configuration/telemetry: all runtime paths must use `logger.FromContext(ctx)` and `config.FromContext(ctx)` and avoid globals.
- API standards: versioned routes, pagination, ETag preconditions, problem+json errors.

## Non‑Goals (Out of Scope)

- Building a chat UI or editor UX; this PRD focuses on APIs/CLI and YAML.
- OCR, document conversion pipelines beyond supported source types.
- Reranking model selection UX and hybrid retrieval; advanced rerankers can be considered post‑MVP.
- Cross‑project sharing or marketplace of knowledge bases.

## Release Scope

MVP includes YAML resources, scope/precedence, ingestion (manual only), query APIs, minimal CLI surface, runtime prompt injection, observability, and support for dense retrieval with simple tag filters. Deferred to post‑MVP: background sync/schedules, hybrid retrieval, advanced filter DSL, external blob stores and cascade purge, deep CLI tooling (`inspect`, `--watch`, `--expand`).

## Success Metrics

- ≥1 knowledge base defined and used in at least N active workflows within 30 days of release (target N to be set by PM).
- Ingestion success rate ≥ 99% weekly on manual runs; failed ingests expose actionable error details in logs/metrics.
- Query error rate ≤ 0.5% weekly (4xx/5xx excluding client misuse).
- Median freshness from source change to availability ≤ X hours (target set during rollout planning).
- Observability coverage: all endpoints and pipelines emit metrics, structured logs, and spans.

## Risks and Mitigations

- Rate limiting and cost from embedding providers → batching, retries with jitter, and clear knobs for batch size/concurrency.
- Vector schema/index migrations risk downtime → `ensure_index` option and migration guidance; opt‑out when needed.
- Retrieval latency impacting user experience → top_k/min_score controls, optional caching, and monitoring of query latency.
- Misconfiguration in YAML leading to failed jobs → strict validation and deterministic precedence; rely on logs/metrics for visibility.

## Appendix

- Reference: `tasks/prd-embedding/_techspec.md` (architecture, APIs, CLI, testing approach, monitoring, libraries assessment).
- Sample YAML snippets for project/workflow/task/agent already included in Tech Spec.

## Acceptance Criteria

AC1. From a fresh project, I can declare an embedder, a vector DB, and a knowledge base in project YAML; validation errors are descriptive when misconfigured.

AC2. In a workflow, I can define a workflow‑scoped knowledge base and bind it from a task or agent; resolution follows precedence rules.

AC3. I can ingest documents from at least two source kinds (e.g., pdf URL and markdown glob) using CLI and API; ingestion completes successfully without separate job polling.

AC4. I can query a knowledge base via API/CLI and receive top‑k results with metadata, stable ordering, and scores; setting `min_score` filters low‑signal results.

AC5. When I delete a knowledge base, vectors are removed from the vector store (vector‑only deletion); no external blob purge is attempted.

AC6. Observability is present: metrics, structured logs, and traces for ingestion and retrieval are visible in standard dashboards or logs.

AC7. Documentation includes YAML examples, CLI walkthroughs, and API route descriptions consistent with Compozy standards.
