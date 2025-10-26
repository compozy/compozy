# Docs Plan (Referenced): Compozy GO SDK

## Goals

- Create a NEW top-level docs section "SDK" alongside Core, CLI Reference, Schema Definition, and API Reference (do not modify docs/ in this PR; this is the implementation plan the docs team will apply).
- Keep content DRY by linking back to the canonical PRD modules under `tasks/prd-sdk/` for technical details.
- Provide precise file paths, outlines, and nav updates required for the docs site.

## Top-level Section Plan (Navigation Tabs)

- Add "SDK" to the docs root navigation tabs next to: Core, CLI Reference, Schema Definition, API Reference.
- Files to be updated by docs team (not in this PR):
  - `docs/content/docs/meta.json`: add "sdk" to `pages` array (after "core").
  - Create `docs/content/docs/sdk/meta.json` with:
    ```json
    {
      "title": "SDK",
      "description": "Compozy GO SDK documentation",
      "icon": "Code2",
      "root": true,
      "pages": [
        "overview",
        "getting-started",
        "architecture",
        "entities",
        "builders",
        "examples",
        "migration",
        "testing",
        "troubleshooting"
      ]
    }
    ```
  - No changes are required to `docs/source.config.ts` beyond content sync.

## New Pages (SDK Section)

- docs/content/docs/sdk/overview.mdx (new)
  - Purpose: What the SDK is and why (ref `tasks/prd-sdk/01-executive-summary.md`).
  - Outline: value props, when to use SDK vs YAML, links to migration and entities.

- docs/content/docs/sdk/getting-started.mdx (new)
  - Purpose: Quick start with context-first pattern (ref `tasks/prd-sdk/05-examples.md`).
  - Outline: context setup, first workflow, build/deploy, embedded server note.

- docs/content/docs/sdk/architecture.mdx (new)
  - Purpose: High-level SDK → Engine integration (ref `tasks/prd-sdk/02-architecture.md`).
  - Outline: workspace, imports, context-first, integration layer overview.

- docs/content/docs/sdk/entities.mdx (new)
  - Purpose: Inventory of 16 categories / 30 builders (ref `tasks/prd-sdk/03-sdk-entities.md`).
  - Outline: list with one-line summaries; deep links to PRD sections.

- docs/content/docs/sdk/builders/*.mdx (new directory)
  - Pages: project, model, workflow, agent (+action), task (9 types), knowledge (5), memory (2), mcp, runtime (+native tools), tool, schema, schedule, monitoring, client, compozy.
  - Each page is a thin wrapper summarizing the builder and linking to `tasks/prd-sdk/03-sdk-entities.md` and code examples in `tasks/prd-sdk/05-examples.md`.

- docs/content/docs/sdk/examples.mdx (new)
  - Purpose: Index of runnable examples (ref `tasks/prd-sdk/05-examples.md`).
  - Outline: commands, context-first reminders, environment notes.

- docs/content/docs/sdk/migration.mdx (new)
  - Purpose: YAML → SDK mapping (ref `tasks/prd-sdk/06-migration-guide.md`).
  - Outline: strategies, hybrid projects, troubleshooting.

- docs/content/docs/sdk/testing.mdx (new)
  - Purpose: Testing guidance (ref `tasks/prd-sdk/07-testing-strategy.md`).
  - Outline: unit/integration/benchmarks, t.Context()/b.Context().

- docs/content/docs/sdk/troubleshooting.mdx (new)
  - Purpose: Common errors & fixes (ref debugging patterns in `tasks/prd-sdk/05-examples.md`).
  - Outline: context missing, ref resolution, validation errors, MCP transport issues.

## Cross-section Link Updates

- Add SDK links in existing sections (thin updates only):
  - Core pages that mention YAML should cross-link to SDK Overview and Migration.
  - API Overview: add “SDK client” link to SDK Overview.
  - CLI Overview: note that SDK is programmatic and link to SDK Getting Started.

## Schema Docs

- No separate schema section for SDK; reference engine schemas and link to builder schema configuration in `tasks/prd-sdk/03-sdk-entities.md`.

## API Docs

- Keep the REST API under API Reference. From SDK pages, link to API Overview where relevant (deploy/execute/status examples in PRD 01 and 06).

## CLI Docs

- No separate CLI for SDK. From SDK pages, link to existing CLI overview when appropriate.

## Tabs/Indexing

- Add SDK to the top tabs (docs team action). Ensure search indexing includes the new SDK folder.

## Acceptance Criteria

- SDK appears as a new top-level tab.
- SDK pages exist with defined outlines and links back to PRD modules; no technical duplication.
- Search and sidebar include SDK content; docs build passes.
- Cross-links from Core/API/CLI to SDK are present where noted.
