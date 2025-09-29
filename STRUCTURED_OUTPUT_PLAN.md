# Structured Output Refactor Report

## 1. Key Findings (pre-change)

- `json_mode` flags on agents/actions and the ad-hoc `StructuredOutput` toggle in the orchestrator created divergent code paths and made it unclear which component was ultimately responsible for enforcing structured replies.
- LangChain-Go integration only flipped `llms.WithJSONMode()`, so providers that support OpenAI's `response_format` capability could not take advantage of schema-native guarantees.
- Response handling compensated for unreliable outputs by injecting pseudo tool-calls, duplicating retry logic and masking the real issue: the model response was never constrained by a schema-aware request.
- Configuration surface area (YAML, DTOs, schemas, docs) exposed `json_mode`, inviting further usage of the workaround pattern.

## 2. Implementation Completed

- **Unified Output Format Abstraction** (`engine/llm/adapter/interface.go`): introduced `OutputFormat` to carry either the default response or a JSON Schema-backed format. `CallOptions` now holds this struct instead of dual booleans.
- **Native Structured Output Requests** (`engine/llm/adapter/langchain_adapter.go`):
  - cached LangChain models by schema fingerprint and reused the new `CreateLLMFactory` response format hook.
  - translated our `schema.Schema` into `openai.ResponseFormatJSONSchemaProperty` and only invoked it for providers that support native structured outputs.
- **Request Builder Pipeline** (`engine/llm/orchestrator/request_builder.go`):
  - derived `OutputFormat` directly from action output schemas when tools are absent and provider support is available.
  - fell back to prompt-level instructions when native structured output is unavailable.
  - replaced JSON-mode logging with an explicit format descriptor for observability.
- **Response Handling** (`engine/llm/orchestrator/response_handler.go`): aligned validation paths with the new format abstraction and updated retry messages to reference structured output expectations.
- **Prompt Builder Simplification** (`engine/llm/prompt_builder.go`): removed `json_mode` awareness and restricted `ShouldUseStructuredOutput` to “provider supports + output schema defined”.
- **Configuration Surface Cleanup**: removed `json_mode` from agent/action config structs, DTOs, router tests, and helper utilities (`engine/agent/action_config.go`, `engine/agent/config.go`, `engine/agent/router/dto.go`, `engine/task/router/dto.go`, `engine/workflow/resolvers.go`, `engine/task2/core/agent_normalizer.go`).
- **Test Adjustments**:
  - Updated unit/integration tests to rely on output schemas instead of JSON mode (`engine/agent/router/exec_test.go`, `engine/llm/orchestrator/response_handler_test.go`, `engine/llm/adapter/langchain_adapter_test.go`).
  - Normalized integration assertions to ignore incidental whitespace differences (`test/integration/worker/basic/basic_helpers.go`).
  - Regenerated expectations where fixtures reference structured output (`test/integration/worker/basic/fixtures/*`).

## 3. Remaining Work & Plan

1. **Schema & OpenAPI regeneration**
   - Update JSON schema assets (`schemas/*.json`) and regenerated Swagger/Go docs to drop `json_mode` fields and document the new structured output behaviour.
2. **Documentation sweep**
   - Replace `json_mode` references across README/MDX content (agents, tasks, examples) with guidance on defining `output` schemas and explaining provider prerequisites.
   - Provide migration guidance in developer docs noting that actions now require explicit schemas to opt into structured outputs.
3. **Examples & samples**
   - Audit example workflows and configs under `examples/` to remove `json_mode` and, where appropriate, add sample output schemas that demonstrate native structured outputs.
4. **Validation tooling**
   - Consider adding a compile-time validation that rejects `json_mode` if it still appears in project configs, guiding users toward schema definitions instead.

## 4. Verification

- `go test ./...`
- Focused suites: `go test ./engine/llm/...`, `go test ./engine/agent/...`, `go test ./test/integration/worker/basic`

The codebase now routes structured-output intent through a single abstraction, leverages provider-native enforcement for OpenAI-compatible models, and no longer exposes `json_mode` in runtime configuration. Documentation and schema assets remain to be updated to reflect the new contract.
