# Builder Outline: Runtime & Native Tools
- **Purpose:** Explain `sdk/runtime` builders for process runtime and `NativeTools`. Source: tasks/prd-sdk/03-sdk-entities.md §"Runtime Configuration".
- **Audience:** Platform engineers operating Compozy deployments.
- **Sections:**
  1. Runtime configuration options (workers, concurrency, scaling) referencing method list.
  2. Native tools registration and lifecycle referencing 03-sdk-entities.md native tools subsection.
  3. Environment dependencies (database, temporal, redis) referencing 02-architecture.md runtime requirements.
  4. Observability & diagnostics referencing monitoring builder and _techspec.md.
  5. Deployment patterns (embedded vs remote) referencing 06-migration-guide.md hybrid notes.
- **Content Sources:** 03-sdk-entities.md, 02-architecture.md, 04-implementation-plan.md runtime tasks.
- **Cross-links:** Compozy builder page, CLI runtime commands, troubleshooting page.
- **Examples:** `sdk/examples/07_embedded_runtime.go`, `10_hybrid_runner.go`.
