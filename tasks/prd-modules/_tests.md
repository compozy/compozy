# Tests Plan (Referenced): Compozy v2 Go SDK

## Guiding Principles

- Follow `.cursor/rules/test-standards.mdc`; use `t.Context()` and avoid `context.Background()` in tests.
- Apply patterns defined in `tasks/prd-modules/07-testing-strategy.md`.

## Coverage Matrix

- Map PRD acceptance criteria to package tests per `tasks/prd-modules/07-testing-strategy.md`.
- Ensure 9 task types, unified Signal, and native tools have representative tests.

## Unit Tests

- Target: builders under `v2/*` (when implemented) using table-driven tests.
- Critical scenarios (reference only; details in PRD):
  - Validation errors aggregation (ref `tasks/prd-modules/02-architecture.md`, error strategy)
  - Context-first propagation (ref `tasks/prd-modules/02-architecture.md`)
  - Task config construction parity with engine (ref `tasks/prd-modules/03-sdk-entities.md`)

## Integration Tests

- SDK → Engine registration and execution flows (ref `tasks/prd-modules/07-testing-strategy.md`).
- External integrations by env-gated tests: MCP, Redis, Postgres/pgvector (ref `tasks/prd-modules/07-testing-strategy.md`).

## Fixtures & Testdata

- Reuse engine testdata where possible; see references in `tasks/prd-modules/07-testing-strategy.md`.

## Mocks & Stubs

- External providers only (e.g., HTTP MCP endpoints); keep internal logic pure (ref `tasks/prd-modules/07-testing-strategy.md`).

## API Contract Assertions

- If HTTP client is used, keep parity with existing server responses; examples in `tasks/prd-modules/06-migration-guide.md`.

## Observability Assertions

- Verify metrics/logs/spans as outlined in `tasks/prd-modules/07-testing-strategy.md`.

## Performance & Limits

- Benchmarks use `b.Context()` (Go 1.25+); targets are in `tasks/prd-modules/07-testing-strategy.md`.

## CLI Tests (Goldens)

- Only where CLI wraps SDK workflows; golden locations as per repo conventions.

## Exit Criteria

- All referenced tests exist and pass locally; CI config updated if needed.
