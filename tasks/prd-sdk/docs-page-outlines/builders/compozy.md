# Builder Outline: Compozy Embedded Engine
- **Purpose:** Document `sdk/compozy` builder for embedding the engine. Source: tasks/prd-sdk/03-sdk-entities.md §"Compozy Embedded Engine".
- **Audience:** Teams embedding Compozy into existing services.
- **Sections:**
  1. Builder responsibilities (wrapping project config, server lifecycle) referencing method list.
  2. Infrastructure dependencies (database, temporal, redis) referencing 02-architecture.md.
  3. Lifecycle methods (Start, Stop, Wait) referencing 03-sdk-entities.md usage example.
  4. Execution APIs (ExecuteWorkflow) referencing 03-sdk-entities.md.
  5. Deployment patterns & readiness checks referencing 04-implementation-plan.md operations tasks.
- **Content Sources:** 03-sdk-entities.md, 02-architecture.md, 04-implementation-plan.md.
- **Cross-links:** Runtime builder, CLI deployment doc, troubleshooting for runtime errors.
- **Examples:** `sdk/examples/07_embedded_runtime.go`, `10_hybrid_runner.go`.
