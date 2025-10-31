# Tests Plan – SDK v2 Compozy

## Guiding Principles

- Follow `.cursor/rules/test-standards.mdc`, using `t.Context()` and context-backed logger/config helpers.
- Prefer table-driven tests with testify `require` for setup validation and `assert` for output comparisons.
- Use generated code fixtures only through documented codegen pipeline to avoid drift.

## Coverage Matrix

- Pure Go bootstrapping (success criteria) → `sdk/compozy/app_test.go`
- Mode awareness default/override → `sdk/compozy/mode_test.go`
- Resource validation & dependency graph → `sdk/compozy/resources/graph_test.go`
- Client integrations (ExecuteWorkflow/Agent/Task) → `sdk/compozy/execution/client_test.go`
- YAML loaders & hybrid mode → `sdk/compozy/config/yaml_loader_test.go`
- Code generation pipeline → `sdk/compozy/codegen/generator_test.go`
- Migration guidance smoke → `sdk/compozy/migration/example_compat_test.go`

## Unit Tests

- sdk/compozy/app_test.go
  - Should construct `App` with minimal options (`WithWorkflow`, `WithAgent`) and execute workflow sync.
  - Should propagate context, logger, and config from parent ctx.
- sdk/compozy/options_test.go
  - Should apply each option (mode, config file, hooks) and validate final state.
- sdk/compozy/resources/graph_test.go
  - Should detect cyclic dependencies and surface wrapped errors.
  - Should build execution order matching spec examples.
- sdk/compozy/codegen/generator_test.go
  - Should generate boilerplate with expected file hashes for sample definitions.

## Integration Tests

- sdk/compozy/integration/standalone_integration_test.go
  - Spin up in-memory execution engine, register workflow + agent, verify `ExecuteWorkflowStream` events.
- sdk/compozy/integration/distributed_integration_test.go
  - Use Redis test container fixture to validate distributed startup + ExecuteAgentStream.
- sdk/compozy/integration/hybrid_yaml_integration_test.go
  - Load YAML resources from temp dir, override programmatically, verify merged configuration.

## Fixtures & Testdata

- `sdk/compozy/testdata/workflows/*.yaml` covering simple, branching, and invalid cases.
- `sdk/compozy/testdata/agents/*.yaml` with minimal agent definitions for validation.
- `sdk/compozy/testdata/codegen/input.yaml` for generator baseline.

## Mocks & Stubs

- Mock `sdk/client` transport interface to simulate success, retries, and failure codes.
- Use fake Redis client interface for distributed tests to avoid flaky network dependencies when not using container fixture.

## API Contract Assertions (if applicable)

- Verify request/response structs match `sdk/client` expectations, including headers and metadata fields.
- Ensure streaming methods emit `OnStart`, `OnData`, `OnComplete`, `OnError` events in doc order.

## Observability Assertions

- Assert structured logs include workflow/agent IDs and correlation IDs via `logger.FromContext`.
- Validate metrics emission (`sdk/compozy/metrics.go`) for startup duration and execution latency using in-memory collector.

## Performance & Limits

- Benchmark `LoadWorkflowsFromDir` with 100 resources to ensure <100ms target.
- Stress test distributed mode with 50 concurrent executions using `testing` benchmarks gated by `-run Integration` tag.

## CLI Tests (Goldens)

- Not applicable; no CLI changes introduced.

## Exit Criteria

- All new tests pass under `gotestsum --format pkgname -- -race -parallel=4 ./sdk/compozy/...`.
- Integration suite runs via `mage integration:sdkCompozy` (to be added) without external dependencies beyond optional Redis container.
- Coverage on `sdk/compozy` exceeds 85%, and race detector passes on integration runs.
