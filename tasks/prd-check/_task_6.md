## markdown

## status: pending

<task_context>
<domain>scripts/markdown</domain>
<type>testing</type>
<scope>migration_validation</scope>
<complexity>high</complexity>
<dependencies>task_5.0</dependencies>
</task_context>

# Task 6.0: Testing and Migration - Comprehensive Tests and Cleanup

## Overview

Complete the refactoring by adding comprehensive testing coverage, validating functional equivalence with the original implementation, performing migration, and cleaning up the old monolithic file. This is the final task that ensures quality and completes the refactoring effort.

This task MUST wait for Task 5.0 to complete, as it validates the entire integrated system.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/go-coding-standards.mdc, @.cursor/rules/architecture.mdc, and @.cursor/rules/test-standards.mdc</critical>

<requirements>
- Achieve > 80% test coverage across all packages
- Verify functional equivalence with original check.go
- All tests must use `t.Context()` instead of `context.Background()`
- Performance must be comparable or better
- No race conditions (verified with -race flag)
- All code passes `make lint` and `make test`
- Proper cleanup and migration of old file
</requirements>

## Subtasks

- [ ] 6.1 Add missing unit tests to achieve 80%+ coverage
- [ ] 6.2 Add comprehensive integration tests
- [ ] 6.3 Add performance comparison tests
- [ ] 6.4 Validate functional equivalence with original implementation
- [ ] 6.5 Run full test suite and fix any issues
- [ ] 6.6 Update build system and documentation
- [ ] 6.7 Migrate old check.go and complete cleanup

## Implementation Details

### 6.1 Coverage Analysis and Missing Tests

**Step 1: Generate Coverage Report**

```bash
# Generate coverage for all packages
go test -coverprofile=coverage.out ./scripts/markdown/...

# View coverage by package
go tool cover -func=coverage.out

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html
```

**Step 2: Identify Low Coverage Areas**

```bash
# Find packages with < 80% coverage
go tool cover -func=coverage.out | awk '$3 < 80.0 {print}'
```

**Step 3: Add Missing Tests**

Focus on:

- **Error paths**: Ensure all error conditions are tested
- **Edge cases**: Boundary conditions, empty inputs, etc.
- **Concurrency**: Race conditions, goroutine leaks
- **Resource cleanup**: File handles, context cancellation
- **Integration points**: Layer boundaries

### 6.2 Comprehensive Integration Tests

**integration_test.go**:

```go
// +build integration

package check_test

import (
    "context"
    "os"
    "path/filepath"
    "testing"

    "scripts/markdown/cmd/check"
    "scripts/markdown/core/models"
)

func TestCompleteWorkflow_SmallDataset(t *testing.T) {
    ctx := t.Context()

    // Setup test data
    testDir := setupTestIssues(t)
    defer os.RemoveAll(testDir)

    // Create config
    config := models.Config{
        PR:              "test-123",
        IssuesDir:       testDir,
        DryRun:          true, // Don't actually execute IDE tools
        Concurrent:      2,
        BatchSize:       2,
        IDE:             "codex",
        Model:           "gpt-5-codex",
        Grouped:         true,
        TailLines:       5,
        ReasoningEffort: "medium",
    }

    // Execute workflow
    container, err := check.NewContainer(ctx)
    if err != nil {
        t.Fatalf("failed to create container: %v", err)
    }
    defer container.Cleanup(ctx)

    useCase := container.SolveIssuesUseCase()
    if err := useCase.Execute(ctx, config); err != nil {
        t.Fatalf("workflow execution failed: %v", err)
    }

    // Verify artifacts
    verifyGroupedSummaries(t, testDir)
    verifyPromptFiles(t, config.PR)
}

func TestWorkflow_WithCancellation(t *testing.T) {
    ctx, cancel := context.WithCancel(t.Context())

    // Setup test data
    testDir := setupTestIssues(t)
    defer os.RemoveAll(testDir)

    config := models.Config{
        PR:         "test-cancel",
        IssuesDir:  testDir,
        DryRun:     false,
        Concurrent: 1,
        BatchSize:  1,
        IDE:        "codex",
    }

    // Start workflow
    container, err := check.NewContainer(ctx)
    if err != nil {
        t.Fatalf("failed to create container: %v", err)
    }
    defer container.Cleanup(ctx)

    // Cancel after short delay
    go func() {
        time.Sleep(100 * time.Millisecond)
        cancel()
    }()

    useCase := container.SolveIssuesUseCase()
    err = useCase.Execute(ctx, config)

    // Should return context canceled error
    if err == nil || !errors.Is(err, context.Canceled) {
        t.Errorf("expected context canceled error, got: %v", err)
    }

    // Verify cleanup happened
    verifyCleanup(t, config.PR)
}

func TestWorkflow_ConcurrentExecution(t *testing.T) {
    ctx := t.Context()

    // Setup test data with multiple issues
    testDir := setupLargeTestIssues(t, 10)
    defer os.RemoveAll(testDir)

    config := models.Config{
        PR:         "test-concurrent",
        IssuesDir:  testDir,
        DryRun:     true,
        Concurrent: 4,
        BatchSize:  2,
        IDE:        "codex",
    }

    // Execute workflow
    container, err := check.NewContainer(ctx)
    if err != nil {
        t.Fatalf("failed to create container: %v", err)
    }
    defer container.Cleanup(ctx)

    useCase := container.SolveIssuesUseCase()
    if err := useCase.Execute(ctx, config); err != nil {
        t.Fatalf("workflow execution failed: %v", err)
    }

    // Verify all jobs completed
    verifyAllJobsCompleted(t, config.PR, 5) // 10 issues / 2 per batch = 5 jobs
}

func setupTestIssues(t *testing.T) string {
    t.Helper()
    dir := t.TempDir()

    // Create sample issue files
    issues := []struct {
        name    string
        content string
    }{
        {
            name: "001-issue.md",
            content: `# Issue 1
**File:** \`path/to/file.go:10\`
This is a test issue.`,
        },
        {
            name: "002-issue.md",
            content: `# Issue 2
**File:** \`path/to/file.go:20\`
Another test issue.`,
        },
    }

    for _, issue := range issues {
        path := filepath.Join(dir, issue.name)
        if err := os.WriteFile(path, []byte(issue.content), 0644); err != nil {
            t.Fatalf("failed to create test issue: %v", err)
        }
    }

    return dir
}
```

### 6.3 Performance Comparison Tests

**performance_test.go**:

```go
// +build performance

package check_test

import (
    "context"
    "testing"
    "time"

    "scripts/markdown/cmd/check"
    "scripts/markdown/core/models"
)

func BenchmarkWorkflow_SmallDataset(b *testing.B) {
    ctx := context.Background()
    testDir := setupTestIssues(b)
    defer os.RemoveAll(testDir)

    config := models.Config{
        PR:         "bench-small",
        IssuesDir:  testDir,
        DryRun:     true,
        Concurrent: 1,
        BatchSize:  3,
        IDE:        "codex",
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        container, err := check.NewContainer(ctx)
        if err != nil {
            b.Fatalf("failed to create container: %v", err)
        }

        useCase := container.SolveIssuesUseCase()
        if err := useCase.Execute(ctx, config); err != nil {
            b.Fatalf("workflow execution failed: %v", err)
        }

        container.Cleanup(ctx)
    }
}

func BenchmarkWorkflow_LargeDataset(b *testing.B) {
    ctx := context.Background()
    testDir := setupLargeTestIssues(b, 50)
    defer os.RemoveAll(testDir)

    config := models.Config{
        PR:         "bench-large",
        IssuesDir:  testDir,
        DryRun:     true,
        Concurrent: 4,
        BatchSize:  5,
        IDE:        "codex",
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        container, err := check.NewContainer(ctx)
        if err != nil {
            b.Fatalf("failed to create container: %v", err)
        }

        useCase := container.SolveIssuesUseCase()
        if err := useCase.Execute(ctx, config); err != nil {
            b.Fatalf("workflow execution failed: %v", err)
        }

        container.Cleanup(ctx)
    }
}

func BenchmarkConcurrentExecution(b *testing.B) {
    ctx := context.Background()
    testDir := setupLargeTestIssues(b, 20)
    defer os.RemoveAll(testDir)

    concurrencyLevels := []int{1, 2, 4, 8}

    for _, concurrency := range concurrencyLevels {
        b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
            config := models.Config{
                PR:         fmt.Sprintf("bench-conc-%d", concurrency),
                IssuesDir:  testDir,
                DryRun:     true,
                Concurrent: concurrency,
                BatchSize:  2,
                IDE:        "codex",
            }

            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                container, err := check.NewContainer(ctx)
                if err != nil {
                    b.Fatalf("failed to create container: %v", err)
                }

                useCase := container.SolveIssuesUseCase()
                if err := useCase.Execute(ctx, config); err != nil {
                    b.Fatalf("workflow execution failed: %v", err)
                }

                container.Cleanup(ctx)
            }
        })
    }
}

// Memory allocation benchmarks
func BenchmarkMemoryAllocation(b *testing.B) {
    ctx := context.Background()
    testDir := setupTestIssues(b)
    defer os.RemoveAll(testDir)

    config := models.Config{
        PR:         "bench-memory",
        IssuesDir:  testDir,
        DryRun:     true,
        Concurrent: 1,
        BatchSize:  3,
        IDE:        "codex",
    }

    b.ReportAllocs()
    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        container, err := check.NewContainer(ctx)
        if err != nil {
            b.Fatalf("failed to create container: %v", err)
        }

        useCase := container.SolveIssuesUseCase()
        if err := useCase.Execute(ctx, config); err != nil {
            b.Fatalf("workflow execution failed: %v", err)
        }

        container.Cleanup(ctx)
    }
}
```

### 6.4 Functional Equivalence Validation

**equivalence_test.go**:

```go
// +build equivalence

package check_test

import (
    "context"
    "os"
    "os/exec"
    "path/filepath"
    "testing"

    "scripts/markdown/cmd/check"
    "scripts/markdown/core/models"
)

// TestFunctionalEquivalence compares outputs between old and new implementations
func TestFunctionalEquivalence(t *testing.T) {
    ctx := t.Context()

    // Setup test data
    testDir := setupTestIssues(t)
    defer os.RemoveAll(testDir)

    config := models.Config{
        PR:         "equiv-test",
        IssuesDir:  testDir,
        DryRun:     true,
        Concurrent: 1,
        BatchSize:  2,
        IDE:        "codex",
        Grouped:    true,
    }

    // Execute new implementation
    newOutputDir := t.TempDir()
    config.PR = "equiv-new"
    if err := executeNewImplementation(ctx, config); err != nil {
        t.Fatalf("new implementation failed: %v", err)
    }

    // Execute old implementation (if available for comparison)
    // This would require keeping the old check.go temporarily
    oldOutputDir := t.TempDir()
    if err := executeOldImplementation(ctx, config, oldOutputDir); err != nil {
        t.Logf("old implementation not available for comparison: %v", err)
        t.Skip("skipping equivalence test - old implementation not available")
    }

    // Compare outputs
    compareOutputs(t, oldOutputDir, newOutputDir)
}

func executeNewImplementation(ctx context.Context, config models.Config) error {
    container, err := check.NewContainer(ctx)
    if err != nil {
        return err
    }
    defer container.Cleanup(ctx)

    useCase := container.SolveIssuesUseCase()
    return useCase.Execute(ctx, config)
}

func executeOldImplementation(ctx context.Context, config models.Config, outputDir string) error {
    // This would execute the backed-up old check.go
    // For demonstration purposes only
    cmd := exec.CommandContext(ctx, "go", "run", "scripts/markdown/check.go.bak",
        "--pr", config.PR,
        "--issues-dir", config.IssuesDir,
        "--dry-run",
        "--concurrent", fmt.Sprintf("%d", config.Concurrent),
        "--batch-size", fmt.Sprintf("%d", config.BatchSize),
        "--ide", config.IDE,
    )
    return cmd.Run()
}

func compareOutputs(t *testing.T, oldDir, newDir string) {
    t.Helper()

    // Compare grouped summaries
    compareFiles(t, filepath.Join(oldDir, "grouped"), filepath.Join(newDir, "grouped"))

    // Compare prompt files
    compareFiles(t, filepath.Join(oldDir, "prompts"), filepath.Join(newDir, "prompts"))
}

func compareFiles(t *testing.T, oldPath, newPath string) {
    t.Helper()

    oldFiles, err := filepath.Glob(filepath.Join(oldPath, "*"))
    if err != nil {
        t.Fatalf("failed to list old files: %v", err)
    }

    newFiles, err := filepath.Glob(filepath.Join(newPath, "*"))
    if err != nil {
        t.Fatalf("failed to list new files: %v", err)
    }

    if len(oldFiles) != len(newFiles) {
        t.Errorf("file count mismatch: old=%d, new=%d", len(oldFiles), len(newFiles))
    }

    // Compare file contents (simplified)
    for _, oldFile := range oldFiles {
        baseName := filepath.Base(oldFile)
        newFile := filepath.Join(newPath, baseName)

        oldContent, err := os.ReadFile(oldFile)
        if err != nil {
            t.Fatalf("failed to read old file: %v", err)
        }

        newContent, err := os.ReadFile(newFile)
        if err != nil {
            t.Fatalf("failed to read new file: %v", err)
        }

        // Compare (allowing for minor formatting differences)
        if !contentEquivalent(oldContent, newContent) {
            t.Errorf("content mismatch for %s", baseName)
        }
    }
}
```

### 6.5 Full Test Suite Execution

**test_runner.sh**:

```bash
#!/bin/bash
set -e

echo "Running full test suite..."

# 1. Run unit tests with coverage
echo "Running unit tests..."
gotestsum --format pkgname -- -race -coverprofile=coverage.out ./scripts/markdown/...

# 2. Check coverage threshold
echo "Checking coverage..."
COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
if (($(echo "$COVERAGE < 80" | bc -l))); then
  echo "Coverage is below 80%: ${COVERAGE}%"
  exit 1
fi
echo "Coverage: ${COVERAGE}%"

# 3. Run integration tests
echo "Running integration tests..."
go test -v -tags=integration ./scripts/markdown/...

# 4. Run linter
echo "Running linter..."
golangci-lint run --fix --allow-parallel-runners ./scripts/markdown/...

# 5. Run performance benchmarks
echo "Running benchmarks..."
go test -bench=. -benchmem -tags=performance ./scripts/markdown/... > benchmark_results.txt

echo "All tests passed!"
```

### 6.6 Update Build System and Documentation

**Update Makefile**:

```makefile
# Add new check command
.PHONY: check-refactored
check-refactored:
	go run scripts/markdown/cmd/check/main.go $(ARGS)

# Add test target for refactored code
.PHONY: test-check
test-check:
	gotestsum --format pkgname -- -race -parallel=4 ./scripts/markdown/...

# Add coverage target
.PHONY: coverage-check
coverage-check:
	go test -coverprofile=coverage.out ./scripts/markdown/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
```

**Update README or docs**:

````markdown
## Markdown Check Tool

Refactored implementation with clean architecture.

### Usage

```bash
# Interactive mode
go run scripts/markdown/cmd/check/main.go

# With flags
go run scripts/markdown/cmd/check/main.go --pr 259 --concurrent 4 --batch-size 3

# Dry run
go run scripts/markdown/cmd/check/main.go --pr 259 --dry-run
```
````

### Architecture

The tool follows clean architecture with these layers:

- `cmd/check/` - Application entry point and CLI
- `core/` - Business logic (services, use cases, domain models)
- `infrastructure/` - External dependencies (file system, command execution)
- `ui/` - User interface (forms, Bubble Tea components)
- `shared/` - Shared utilities

### Testing

```bash
# Run unit tests
make test-check

# Run with coverage
make coverage-check

# Run integration tests
go test -v -tags=integration ./scripts/markdown/...
```

````

### 6.7 Migration and Cleanup

**Migration steps**:

1. **Backup old file**:
```bash
cp scripts/markdown/check.go scripts/markdown/check.go.bak
````

2. **Update build scripts** to use new implementation

3. **Run equivalence tests** to ensure functional parity

4. **Update documentation** with new usage patterns

5. **Remove old file** (after validation period):

```bash
rm scripts/markdown/check.go.bak
```

6. **Update git history**:

```bash
git add scripts/markdown/
git commit -m "refactor(check): complete clean architecture refactoring

- Broke 3,055-line monolithic check.go into clean architecture
- Implemented SOLID principles throughout
- Added comprehensive test coverage (>80%)
- Validated functional equivalence
- Improved maintainability and extensibility

BREAKING CHANGE: None - CLI interface is preserved"
```

### Relevant Files

**Files to Create/Update**:

- `scripts/markdown/cmd/check/integration_test.go`
- `scripts/markdown/cmd/check/performance_test.go`
- `scripts/markdown/cmd/check/equivalence_test.go`
- `scripts/markdown/test_runner.sh`
- `Makefile` (update)
- `README.md` or `docs/check-tool.md` (update)

**Files to Archive**:

- `scripts/markdown/check.go` → `scripts/markdown/check.go.bak`

**Files to Eventually Remove**:

- `scripts/markdown/check.go.bak` (after validation period)

## Deliverables

- [ ] Test coverage > 80% across all packages
- [ ] All integration tests passing
- [ ] Performance benchmarks completed and documented
- [ ] Functional equivalence validated
- [ ] Full test suite (`make test`) passing
- [ ] Linting (`make lint`) passing with zero errors
- [ ] Build system updated
- [ ] Documentation updated
- [ ] Old file backed up and eventually removed
- [ ] Migration complete

## Tests

### Coverage Tests

- [ ] Achieve > 80% coverage in `core/models/`
- [ ] Achieve > 80% coverage in `core/services/`
- [ ] Achieve > 80% coverage in `core/usecases/`
- [ ] Achieve > 80% coverage in `infrastructure/`
- [ ] Achieve > 70% coverage in `ui/` (lower threshold for UI)
- [ ] Achieve > 80% coverage in `cmd/check/`

### Integration Tests

- [ ] Complete workflow with small dataset
- [ ] Workflow with context cancellation
- [ ] Concurrent execution with multiple jobs
- [ ] Dry-run mode
- [ ] Grouped summaries generation
- [ ] Error handling and recovery

### Performance Tests

- [ ] Benchmark small dataset processing
- [ ] Benchmark large dataset processing
- [ ] Benchmark concurrent execution at different levels
- [ ] Memory allocation benchmarks
- [ ] Compare performance with old implementation

### Equivalence Tests

- [ ] Output files match between old and new
- [ ] Grouped summaries are equivalent
- [ ] Prompt files are equivalent
- [ ] CLI interface is preserved
- [ ] All features work identically

## Success Criteria

### Functional Requirements

- [ ] All original functionality preserved
- [ ] No regressions in behavior
- [ ] Performance is comparable or better
- [ ] CLI interface unchanged
- [ ] Error handling improved

### Quality Requirements

- [ ] Test coverage > 80%
- [ ] All tests pass: `make test`
- [ ] All linting passes: `make lint`
- [ ] No race conditions: verified with `-race`
- [ ] No goroutine leaks
- [ ] No file handle leaks
- [ ] Memory usage reasonable

### Documentation Requirements

- [ ] Usage documentation updated
- [ ] Architecture documented
- [ ] Migration guide provided
- [ ] Testing guide provided
- [ ] Build system documented

### Migration Requirements

- [ ] Old file backed up
- [ ] Equivalence validated
- [ ] Build system updated
- [ ] Team informed of changes
- [ ] Rollback plan documented

## Implementation Notes

### Order of Implementation

1. Generate coverage report and identify gaps
2. Add missing unit tests
3. Write integration tests
4. Write performance benchmarks
5. Run equivalence validation
6. Update build system
7. Update documentation
8. Execute migration
9. Final validation: `make fmt && make lint && make test`

### Key Design Decisions

- **80% coverage threshold**: Industry standard for quality
- **Integration tests**: Verify layer interactions
- **Performance tests**: Ensure no regression
- **Equivalence tests**: Validate functional parity
- **Graceful migration**: Backup, validate, then remove

### Common Pitfalls to Avoid

- ❌ Don't skip edge case testing
- ❌ Don't ignore race conditions
- ❌ Don't skip performance validation
- ❌ Don't remove old file too early
- ❌ Don't forget to update documentation

### Testing Strategy

- **Comprehensive coverage**: Test all code paths
- **Integration testing**: Test real workflows
- **Performance testing**: Benchmark against old version
- **Equivalence testing**: Validate functional parity
- **Stress testing**: Concurrent execution, large datasets

## Dependencies

**Blocks**: Nothing (this is the final task)

**Blocked By**: Task 5.0 (Application Wiring)

**Parallel With**: None (must wait for complete integration)

## Estimated Effort

**Size**: Large (L)
**Duration**: 3+ days
**Complexity**: High - Comprehensive testing, validation, and migration
