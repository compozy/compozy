# Docs Plan Template

## Goals

- Define documentation work required to land the feature across the docs site.
- Provide precise file paths, page outlines, and cross-link updates.

## New/Updated Pages

- [path/to/page.mdx] (new|update)
  - Purpose: [short purpose]
  - Outline:
    - [section]
    - [section]
  - Links: [related pages]

## Schema Docs

- [path/to/schema-page.mdx] (new|update)
  - Renders `schemas/[file].json`
  - Notes: [what to highlight]

## API Docs

- [path/to/api/page.mdx] (new|update)
  - Endpoints to include
  - Example requests/responses

## CLI Docs

- [path/to/cli/page.mdx] (new|update)
  - Commands and flags
  - Output examples

## Cross-page Updates

- [path/to/page.mdx] (update)
  - [brief update note]

## Navigation & Indexing

- Update `docs/source.config.ts` for grouping and sidebar order.

## Acceptance Criteria

- Pages exist with correct outlines and links.
- Swagger renders endpoints referenced.
- Docs dev server builds without missing routes.
