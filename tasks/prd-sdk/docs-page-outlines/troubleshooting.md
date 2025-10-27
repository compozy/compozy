# Page Outline: SDK Troubleshooting
- **Purpose:** Catalog common errors, diagnostics, and fixes. Sources: task_55.md troubleshooting deliverable, 06-migration-guide.md FAQ, 05-examples.md debugging notes.
- **Audience:** Support engineers and developers debugging SDK projects.
- **Prerequisites:** Completed Getting Started guide.
- **Key Sections:**
  1. Error taxonomy (context missing, registry conflicts, builder validation) referencing task_55.md §"Common Failures".
  2. Diagnostics workflow (logging, `compozy config diagnostics`) referencing task_55.md §"Troubleshooting Workflow".
  3. MCP & runtime issues (transport, credential problems) referencing 03-sdk-entities.md mcp + runtime sections.
  4. Migration-specific errors (YAML parity, schema mismatches) referencing task_54.md advanced migration issues.
  5. Support escalation checklist (logs, config bundles) referencing _techspec.md ops appendix.
- **Cross-links:** Core diagnostics, CLI config commands, SDK testing page.
- **Examples Strategy:** Reference example failure modes in `sdk/examples/README` sections; link to reproduction scripts when available.
- **Notes:** Provide anchored link to knowledge base articles when published; include “last updated” metadata for quick maintenance.
