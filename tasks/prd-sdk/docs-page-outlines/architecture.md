# Page Outline: SDK Architecture
- **Purpose:** Explain how the SDK integrates with the Compozy Engine and runtime services. Sources: tasks/prd-sdk/02-architecture.md (integration diagrams, runtime lifecycle), _techspec.md (context/lifecycle rules).
- **Audience:** Architects and senior engineers designing deployments.
- **Prerequisites:** Read SDK overview and Core architecture page.
- **Key Sections:**
  1. High-level architecture diagram narrative (pull figure references from 02-architecture.md §"System Overview").
  2. Context propagation and dependency attachment (logger/config) referencing _techspec.md "Context-First Pattern".
  3. Engine integration flow (builder outputs → registry → runtime) referencing 04-implementation-plan.md §"Engine Registration".
  4. Deployment topologies (embedded, hybrid SDK+YAML, remote execution) referencing 06-migration-guide.md hybrid notes.
  5. Observability hooks (monitoring builder, metrics) referencing 03-sdk-entities.md monitoring package + 02-architecture.md.
- **Cross-links:** Core runtime internals doc, CLI deployment doc, API reference for remote executor endpoints.
- **Examples Strategy:** Link to architecture-focused examples `sdk/examples/07_embedded_runtime.go` and `08_hybrid_project.go` from 05-examples.md.
- **Notes:** Highlight non-goals (no global singletons) and pointer to troubleshooting for common integration failures.
