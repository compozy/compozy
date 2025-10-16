# Docs Plan Template

## Goals

- Document new execution-level usage fields in API reference and developer guides.
- Provide guidance for interpreting usage data and monitoring metrics.

## New/Updated Pages

- docs/api/executions.mdx (update)
  - Purpose: Document new `usage` object in workflow/task/agent execution responses.
  - Outline:
    - Overview
    - Usage object schema
    - Examples for workflow/task/agent
    - Error handling when usage missing
  - Links: docs/api/agents.mdx, docs/api/tasks.mdx
- docs/how-to/monitor-usage.mdx (new)
  - Purpose: Explain how to monitor LLM usage metrics and alerts.
  - Outline:
    - Prerequisites
    - Metrics overview
    - Grafana dashboard pointers
    - Alert tuning tips
  - Links: docs/reference/metrics.mdx

## Schema Docs

- docs/reference/schemas/execution-usage.mdx (new)
  - Renders `schemas/execution-usage.json`
  - Notes: Highlight nullable fields and provider-specific extensions.

## API Docs

- docs/api/executions.mdx (update)
  - Endpoints to include: GET /executions/workflows/:id, GET /executions/tasks/:id, GET /executions/agents/:id
  - Example requests/responses with usage payload.

## CLI Docs

- docs/cli/executions.mdx (update)
  - Commands and flags: `compozy executions workflows get`, `tasks get`, `agents get`
  - Output examples showing usage tokens.

## Cross-page Updates

- docs/concepts/observability.mdx (update)
  - Note availability of LLM usage metrics and link to monitoring guide.

## Navigation & Indexing

- Update `docs/source.config.ts` to include monitoring guide under Observability and schema doc under Reference.

## Acceptance Criteria

- Updated pages render with new sections.
- Swagger/OpenAPI samples include `usage` object.
- Docs build passes without broken links.
