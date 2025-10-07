# Tests Plan Template

## Guiding Principles

- Follow `.cursor/rules/test-standards.mdc` and project rules.
- Use `t.Run("Should …")`, testify, and context helpers.

## Coverage Matrix

- Map PRD acceptance criteria → concrete test files.

## Unit Tests

- [package]/[file]\_test.go
  - Should …
  - Should …

## Integration Tests

- Gated by env/build tag; deterministic containers if needed.

## Fixtures & Testdata

- List fixtures to add under `engine/[domain]/testdata/`.

## Mocks & Stubs

- External providers only; prefer pure functions for internals.

## API Contract Assertions (if applicable)

- ETag, pagination, problem+json, status codes, swagger parity.

## Observability Assertions

- Metrics/logs/spans presence and labels.

## Performance & Limits

- Deterministic checks for batch sizes/latency/token budgeting as applicable.

## CLI Tests (Goldens)

- Output structure and flags; golden location; UPDATE_GOLDEN toggle.

## Exit Criteria

- All tests exist and pass locally; CI config updated if needed.
