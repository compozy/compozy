# Temporal Standalone Integration Testing Example

## Purpose

Demonstrates how to spin up the embedded Temporal server inside Go integration tests using build tags so the suite stays fast during regular unit test runs.

## Key Concepts

- Configure standalone Temporal for test isolation using environment overrides
- Use `test/helpers` to attach config and logger contexts without globals
- Allocate dynamic ports to avoid collisions in CI
- Guard heavier tests behind the `integration` build tag

## Prerequisites

- Go 1.25.2+
- Bun optional (not required for this workflow)
- Model API key (copy `.env.example`)

## Quick Start

```bash
cd examples/temporal-standalone/integration-testing
cp .env.example .env
make test # runs go test -tags=integration ./tests/...
```

## Test Strategy

- Tests live under `tests/` and are built only when `-tags=integration` is provided
- Each test acquires a free port range, starts the embedded server, runs assertions, and shuts it down with `t.Cleanup`
- UI is disabled for faster startup; persistence uses `:memory:` so no cleanup is required

## Expected Output

- `PASS` for `TestStandaloneServerLifecycle` when the build tag is enabled
- Logs show server startup and shutdown within a few seconds
- Workflow definition (`workflow.yaml`) demonstrates deterministic calculator behavior for additional scenarios

## Troubleshooting

- `address already in use`: reduce test parallelism or widen the random port search range (`findOpenPortRange` helper).
- `missing config`: ensure tests use `helpers.NewTestContext` so `logger.FromContext` and `config.FromContext` are attached.
- Forgot to enable the build tag: run `go test -tags=integration ./tests/...` or `make test` from this directory.

## What's Next

- Integrate the helper pattern into your main integration suite under `test/integration/temporal`
- Pair with the persistence example (`../persistent/`) to validate restart behavior during tests
