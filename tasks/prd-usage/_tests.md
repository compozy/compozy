# Tests Plan Template

## Guiding Principles

- Adhere to `.cursor/rules/test-standards.mdc`: context-aware tests, testify assertions, structured naming.
- Ensure forward-only ingest path is verified; avoid over-engineering Phase 2 backfill paths.

## Coverage Matrix

- PRD R1–R2 → adapter unit tests ensuring usage extraction/defaulting.
- PRD R3–R4 → repository + integration tests verifying persistence and FK integrity.
- PRD R5–R6 → router/API tests confirming `usage` fields in responses.
- PRD R7–R8 → monitoring tests capturing metrics/alert emission thresholds.

## Unit Tests

- `engine/llm/adapter/langchain_adapter_test.go`
  - Should populate `Usage` when GenerationInfo provided.
  - Should set `Usage` to nil and log warning when metadata missing.
- `engine/llm/usage/collector_test.go`
  - Should aggregate token counts across multiple loop iterations.
  - Should finalize with zeroed counts on error paths without panics.
- `engine/infra/postgres/usage_repo_test.go`
  - Should upsert usage row and enforce unique constraint.
  - Should reject orphan records lacking workflow/task IDs.

## Integration Tests

- `test/integration/usage/workflow_usage_test.go`
  - Run workflow with mocked LLM returning usage → verify DB + API response.
- `test/integration/usage/task_usage_test.go`
  - Direct task execution returns usage in `/executions/tasks/:id`.
- `test/integration/usage/missing_metadata_test.go`
  - Provider omits usage → API shows null, metrics increment failure counter.

## Fixtures & Testdata

- Add `engine/llm/usage/testdata/usage_response.json` for adapter tests.
- Add SQL migration fixture for `execution_llm_usage` table in integration setup.

## Mocks & Stubs

- Mock LangChain client to emit deterministic usage counts.
- Use pgxmock for repository tests; integration tests rely on ephemeral Postgres container.

## API Contract Assertions (if applicable)

- Assert new `usage` object presence, nullability, and types.
- Ensure existing fields remain unchanged to protect backward compatibility.

## Observability Assertions

- Validate Prometheus registry contains usage counters with expected labels.
- Check failure counter increments when ingestion fails.

## Performance & Limits

- Add benchmark or load-focused unit test verifying collector adds ≤5% overhead (optional micro-benchmark in `engine/llm/usage`).

## CLI Tests (Goldens)

- Update golden files for `compozy executions workflows get` etc. ensuring usage fields appear.
- Use `UPDATE_GOLDEN` flow per standards.

## Exit Criteria

- All unit/integration tests implemented and passing locally.
- CI includes new integration suite (documented in pipeline config).
- Metrics assertions stable across runs.
