# Docs Plan — Executions Streaming (/stream)

## New/Updated Pages

- docs/content/docs/api/workflows.mdx (update)
  - Purpose: surface new workflow stream endpoint.
  - Outline:
    - Section: Streaming Endpoints
    - Endpoint: `GET /executions/workflows/{exec_id}/stream`
    - Notes: SSE headers, Last-Event-ID, events set, heartbeats, examples.
  - Links: CLI Executions, Schema: execution-usage

- docs/content/docs/api/agents.mdx (update)
  - Purpose: surface new agent stream endpoint.
  - Outline:
    - Section: Streaming Endpoints
    - Endpoint: `GET /executions/agents/{exec_id}/stream`
    - Behavior: structured JSON when output schema exists; otherwise text-only `llm_chunk`.

- docs/content/docs/api/tasks.mdx (update)
  - Purpose: surface new task stream endpoint.
  - Outline:
    - Section: Streaming Endpoints
    - Endpoint: `GET /executions/tasks/{exec_id}/stream`
    - Behavior: structured JSON when output schema exists; otherwise text-only `llm_chunk`.

- docs/content/docs/api/overview.mdx (update)
  - Purpose: introduce streaming overview and client patterns (SSE, EventSource, curl).
  - Outline:
    - Why SSE for executions
    - Event types: `workflow_status`, `tool_call`, `llm_chunk`, `complete`, `error`
    - Reconnect/`Last-Event-ID`
    - Example JS snippet

## Schema Docs

- docs/content/docs/schema/execution-stream-events.mdx (new)
  - Renders event shapes for workflow JSON events and `llm_chunk` semantics.
  - Notes: JSON vs text rule; resume semantics; heartbeats.

## API Docs

- Ensure Swagger includes the three `GET /executions/*/stream` operations with correct `text/event-stream` content type and query/header docs (poll_ms, events, Last-Event-ID).

## CLI Docs

- docs/content/docs/cli/executions.mdx (update)
  - Add a subsection “Streaming From the API” with curl and Node EventSource examples.
  - Clarify difference between CLI `--follow` (polling) vs API SSE.

## Cross-page Updates

- docs/content/docs/core/temporal (add brief link in relevant pages if needed: durable state & queries basics).

## Navigation & Indexing

- Update `docs/content/docs/api/overview.mdx` TOC to include Streaming section.
- Consider a short “Streaming” page in API category if we prefer a standalone hub (optional).

## Acceptance Criteria

- API pages list new `/stream` endpoints with examples.
- Overview contains an SSE quickstart and event catalog.
- CLI page explains how to consume streams programmatically.
- Docs site builds without missing routes; swagger renders updated endpoints.
