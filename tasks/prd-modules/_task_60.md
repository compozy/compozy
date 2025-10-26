## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>ci</domain>
<type>infrastructure</type>
<scope>ci_updates</scope>
<complexity>low</complexity>
<dependencies>task_57,task_58,task_59</dependencies>
</task_context>

# Task 60.0: CI Updates: Workspace + Coverage (S)

## Overview

Update CI/CD pipeline to support Go workspace, run v2 SDK tests, enforce 100% coverage, and integrate benchmark regression detection.

<critical>
- **ALWAYS READ** tasks/prd-modules/07-testing-strategy.md (CI section)
- **MUST** initialize Go workspace in CI: `go work init . ./v2`
- **MUST** enforce 100% test coverage for v2/ packages
- **MUST** run integration tests with testcontainers
- **MUST** detect performance regressions
</critical>

<requirements>
- Update GitHub Actions workflow for Go workspace
- Add v2 SDK test job (unit + integration + benchmarks)
- Enforce 100% coverage requirement
- Setup testcontainers infrastructure
- Add benchmark regression detection
- Update coverage reporting
- Maintain existing CI for main module
</requirements>

## Subtasks

- [ ] 60.1 Update GitHub Actions to initialize Go workspace
- [ ] 60.2 Add v2 SDK unit test job with coverage enforcement
- [ ] 60.3 Add v2 SDK integration test job with testcontainers
- [ ] 60.4 Add v2 SDK benchmark job with regression detection
- [ ] 60.5 Update coverage reporting to include v2/ packages
- [ ] 60.6 Add v2 SDK linting job
- [ ] 60.7 Update PR checks to require v2 tests
- [ ] 60.8 Document CI configuration

## Implementation Details

**Based on:** tasks/prd-modules/07-testing-strategy.md (CI/CD Integration section)

### GitHub Actions Workflow Updates

```yaml
# .github/workflows/test-v2-sdk.yml (NEW)
name: Test v2 SDK

on:
  push:
    branches: [main]
    paths:
      - 'v2/**'
      - 'go.work'
      - '.github/workflows/test-v2-sdk.yml'
  pull_request:
    paths:
      - 'v2/**'
      - 'go.work'

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.25.2'

      - name: Initialize Go Workspace
        run: go work init . ./v2

      - name: Download Dependencies
        run: |
          cd v2
          go mod download

      - name: Run Unit Tests
        run: |
          cd v2
          go test -v -cover -race ./...

      - name: Check Coverage (100% Required)
        run: |
          cd v2
          go test -coverprofile=coverage.out ./...
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          echo "Coverage: $COVERAGE%"
          if awk -v c="$COVERAGE" 'BEGIN {exit !(c < 100)}'; then
            echo "❌ Coverage is $COVERAGE%, must be 100%"
            exit 1
          fi
          echo "✅ Coverage is 100%"

      - name: Upload Coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./v2/coverage.out
          flags: v2-sdk

  integration-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: pgvector/pgvector:latest
        env:
          POSTGRES_PASSWORD: postgres
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

      redis:
        image: redis:latest
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.25.2'

      - name: Initialize Go Workspace
        run: go work init . ./v2

      - name: Run Integration Tests
        run: |
          cd v2
          go test -v -tags=integration ./integration/...
        env:
          POSTGRES_DSN: postgres://postgres:postgres@localhost:5432/test
          REDIS_DSN: redis://localhost:6379
          OPENAI_API_KEY_TEST: ${{ secrets.OPENAI_API_KEY_TEST }}

  benchmarks:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0  # Need history for comparison

      - uses: actions/setup-go@v5
        with:
          go-version: '1.25.2'

      - name: Initialize Go Workspace
        run: go work init . ./v2

      - name: Run Benchmarks
        run: |
          cd v2
          go test -bench=. -benchmem -run=^$ ./... | tee bench-new.txt

      - name: Compare with Baseline
        run: |
          cd v2
          # Download baseline from previous run
          gh run download --name bench-baseline || echo "No baseline"

          # Compare if baseline exists
          if [ -f bench-baseline.txt ]; then
            go install golang.org/x/perf/cmd/benchstat@latest
            benchstat bench-baseline.txt bench-new.txt || exit 1
          fi

      - name: Save New Baseline
        uses: actions/upload-artifact@v3
        with:
          name: bench-baseline
          path: v2/bench-new.txt

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.25.2'

      - name: Initialize Go Workspace
        run: go work init . ./v2

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v3
        with:
          version: latest
          working-directory: v2
          args: --timeout=5m
```

### Coverage Badge Update

```yaml
# Update .github/workflows/coverage-badge.yml
# Add v2 SDK coverage badge alongside main coverage
```

### Relevant Files

- .github/workflows/test-v2-sdk.yml (new)
- .github/workflows/test.yml (existing, keep separate)
- .github/workflows/coverage-badge.yml (update)
- go.work (created by CI)

### Dependent Files

- Task 57.0 deliverable (unit tests)
- Task 58.0 deliverable (integration tests)
- Task 59.0 deliverable (benchmarks)

## Deliverables

- `/Users/pedronauck/Dev/compozy/compozy/.github/workflows/test-v2-sdk.yml` (new file)
  - Unit test job with 100% coverage enforcement
  - Integration test job with testcontainers (Postgres, Redis)
  - Benchmark job with regression detection
  - Lint job for v2/ packages
- Update to `.github/workflows/coverage-badge.yml` (v2 coverage badge)
- Update to repository settings (require v2 tests to pass for PRs)
- Documentation in `v2/docs/ci-configuration.md`

## Tests

CI workflow validation:
- [ ] Trigger workflow manually and verify all jobs pass
- [ ] Unit test job enforces 100% coverage
- [ ] Integration test job starts testcontainers correctly
- [ ] Benchmark job compares with baseline
- [ ] Lint job catches code quality issues
- [ ] Coverage is uploaded to Codecov
- [ ] PR checks require all v2 jobs to pass

Workspace initialization:
- [ ] `go work init . ./v2` succeeds in CI
- [ ] Both modules are testable in workspace
- [ ] Dependencies resolve correctly

Performance:
- [ ] CI jobs complete in < 10 minutes total
- [ ] Testcontainers start reliably
- [ ] Benchmark comparison is accurate

## Success Criteria

- Go workspace is initialized correctly in all CI jobs
- 100% test coverage is enforced (CI fails if < 100%)
- Integration tests run with real services via testcontainers
- Benchmark regression detection prevents performance degradation
- v2 SDK tests are required for PR merge
- Coverage reporting includes v2/ packages
- CI configuration is documented and maintainable
- All v2 CI jobs are green on main branch
- PR workflow requires v2 tests to pass
