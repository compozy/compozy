# Builder Outline: Monitoring
- **Purpose:** Describe `sdk/monitoring` builder for observability configuration. Source: tasks/prd-sdk/03-sdk-entities.md §"Monitoring Configuration".
- **Audience:** SREs and platform engineers.
- **Sections:**
  1. Metrics & tracing options (Prometheus, OTLP) referencing method list.
  2. Log configuration hooks referencing _techspec.md logging mandates.
  3. Alerting integrations referencing 04-implementation-plan.md operations tasks.
  4. Validation & best practices referencing 02-architecture.md observability layer.
  5. Rollout checklist linking to testing/troubleshooting pages.
- **Content Sources:** 03-sdk-entities.md, 02-architecture.md, 04-implementation-plan.md.
- **Cross-links:** Runtime builder, troubleshooting page, Core observability doc.
- **Examples:** `sdk/examples/07_embedded_runtime.go` instrumentation section.
