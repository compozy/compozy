# Builder Outline: Model
- **Purpose:** Describe `sdk/model` builder for provider configuration. Source: tasks/prd-sdk/03-sdk-entities.md §"Model Configuration".
- **Audience:** Engineers configuring LLM/embedding providers.
- **Sections:**
  1. Supported providers & capabilities referencing 03-sdk-entities.md provider matrix.
  2. Configuration methods (WithProvider, WithFallback, WithRateLimits) summarizing method list.
  3. Secrets & config handling referencing 04-implementation-plan.md security notes.
  4. Validation rules & error cases referencing 03-sdk-entities.md validation section.
  5. Integrations with project builder and runtime.
- **Content Sources:** 03-sdk-entities.md, 02-architecture.md provider lifecycle, _techspec.md security guidelines.
- **Cross-links:** Core model registry doc, API rate-limits doc, examples index (parallel tasks example).
- **Examples:** Link to `sdk/examples/02_parallel_tasks.go` and `04_model_failover.go`.
