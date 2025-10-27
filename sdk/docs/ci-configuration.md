# SDK CI Configuration

This document describes the dedicated GitHub Actions workflow that validates the Go SDK.

## Workflow Overview

- **Workflow name:** `SDK Tests` (`.github/workflows/test-sdk.yml`)
- **Triggers:** push and pull requests touching `sdk/**`, `go.work`, or the workflow file, plus manual `workflow_dispatch`
- **Go version:** 1.25.2, matching the repository toolchain

## Jobs

### SDK Unit Tests

- Rebuilds the Go workspace with `go work init . ./sdk` on every run
- Executes `go test -race -covermode=atomic -coverprofile=coverage.out $(go list ./... 2>/dev/null | grep -v '/examples')`
  inside `sdk/` to skip example binaries that depend on unreleased engine packages
- Enforces 100% coverage using `go tool cover` (CI fails if total coverage < 100%)
- Uploads `sdk/coverage.out` to Codecov with the `sdk-unit` flag

### SDK Integration Tests

- Re-initializes the Go workspace and runs `go test -count=1 -tags=integration $(go list ./... 2>/dev/null | grep -v '/examples')`
  - Relies on Testcontainers to provision Postgres and Redis automatically during test execution

### SDK Benchmarks

- Runs `go test -run=^$ -bench=. -benchmem $(go list ./... 2>/dev/null | grep -v '/examples')` and writes results to `sdk/bench.out`
- Uses `go run ./tools/benchcheck --baseline sdk/docs/performance-benchmarks.json --results sdk/bench.out`
  to detect regressions against the curated baseline (20% threshold)
- Publishes `sdk/bench.out` as a workflow artifact for inspection

### SDK Lint

- Rebuilds the workspace and executes `golangci-lint run --timeout=5m` scoped to `sdk/`

## Coverage Reporting

- `.github/workflows/coverage.yml` now generates an additional report via
  `go test -coverprofile=coverage.out $(go list ./... 2>/dev/null | grep -v '/examples')`
  inside `sdk/`, enforces the 100% requirement, and uploads both root (`coverage.out`) and SDK (`sdk/coverage.out`)
  profiles to Codecov.

## Local Verification

```bash
rm -f go.work && go work init . ./sdk
cd sdk
PKGS=$(go list ./... 2> /dev/null | grep -v '/examples')
go test -race -covermode=atomic -coverprofile=coverage.out $PKGS
go tool cover -func=coverage.out
go test -count=1 -tags=integration $PKGS
go test -run=^$ -bench=. -benchmem $PKGS | tee bench.out
cd .. && go run ./tools/benchcheck --baseline sdk/docs/performance-benchmarks.json --results sdk/bench.out
golangci-lint run --timeout=5m ./sdk/...
```

## Updating Benchmark Baselines

1. Run the benchmark job locally: `go test -run=^$ -bench=. -benchmem ./sdk/... | tee sdk/bench.out`
2. Inspect results and ensure no regressions
3. Convert the output into `sdk/docs/performance-benchmarks.json` (same format used by `tools/benchcheck`)
4. Commit the updated JSON alongside any performance improvements

## PR Requirements

- Branch protection can now require the `SDK Tests` workflow for merges; ensure the job is marked as required in GitHub settings.
- Coverage < 100%, benchmark regressions, or lint errors will block merges until resolved.
