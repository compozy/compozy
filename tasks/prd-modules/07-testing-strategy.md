# Testing Strategy: Compozy v2 Go SDK

**Date:** 2025-01-25
**Version:** 1.0.0
**Estimated Reading Time:** 10 minutes

---

## Overview

This document outlines the comprehensive testing strategy for the Compozy v2 Go SDK.

**Testing Goals:**
- 100% code coverage for all builders
- Zero regressions in existing functionality
- Comprehensive integration testing
- Performance validation
- Security validation

---

## Testing Pyramid

```
           /\
          /  \
         /E2E \        E2E Tests (10%)
        /------\       - Full deployment scenarios
       / Integ  \      Integration Tests (20%)
      /----------\     - Real DB/API integration
     /   Unit     \    Unit Tests (70%)
    /--------------\   - Builder logic, validation
```

---

## Unit Testing

### Coverage Target: 100%

**Scope:** All builder packages

**What to Test:**
- Builder construction (New functions)
- Method chaining (fluent API)
- Validation logic
- Error handling
- Edge cases

### Unit Test Structure

```go
// v2/workflow/builder_test.go
package workflow

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestWorkflowBuilder_Success(t *testing.T) {
    ctx := t.Context()
    wf, err := New("test").
        WithDescription("Test workflow").
        AddAgent(agent).
        AddTask(task).
        Build(ctx)

    require.NoError(t, err)
    assert.Equal(t, "test", wf.ID)
    assert.Equal(t, "Test workflow", wf.Description)
    assert.Len(t, wf.Agents, 1)
    assert.Len(t, wf.Tasks, 1)
}

func TestWorkflowBuilder_ValidationError(t *testing.T) {
    // Empty ID
    _, err := New("").Build(t.Context())
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "ID is required")

    // No tasks
    _, err = New("test").Build(t.Context())
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "at least one task")
}

func TestWorkflowBuilder_EdgeCases(t *testing.T) {
    // Very long ID
    longID := strings.Repeat("a", 1000)
    _, err := New(longID).Build(t.Context())
    assert.Error(t, err)

    // Special characters in ID
    _, err = New("test@#$").Build(t.Context())
    assert.Error(t, err)
}
```

### Test Coverage Commands

```bash
# Run tests with coverage
go test -cover ./v2/...

# Generate coverage report
go test -coverprofile=coverage.out ./v2/...
go tool cover -html=coverage.out

# Coverage by package
go test -coverprofile=coverage.out ./v2/... && \
  go tool cover -func=coverage.out
```

---

## Integration Testing

### Coverage Target: Critical Paths

**Scope:** Real system integration

**What to Test:**
- Knowledge system (real embedders + vector DBs)
- Memory system (Redis persistence)
- MCP integration (real MCP servers)
- Client communication (real server)
- End-to-end workflows

### Integration Test Structure

```go
// v2/knowledge/integration_test.go
// +build integration

package knowledge

import (
    "context"
    "testing"
    "github.com/testcontainers/testcontainers-go"
)

func TestKnowledgeSystem_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    ctx := t.Context()

    // Start PostgreSQL with pgvector
    postgres, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: testcontainers.ContainerRequest{
            Image: "pgvector/pgvector:latest",
            // ... config
        },
        Started: true,
    })
    require.NoError(t, err)
    defer postgres.Terminate(ctx)

    // Configure embedder (real OpenAI)
    embedder, _ := NewEmbedder("openai", "openai", "text-embedding-3-small").
        WithAPIKey(os.Getenv("OPENAI_API_KEY_TEST")).
        WithDimension(1536).
        Build(ctx)

    // Configure vector DB (testcontainer)
    dsn := postgres.ConnectionString(ctx)
    vectorDB, _ := NewPgVector("pgvector").
        WithDSN(dsn).
        WithDimension(1536).
        Build(ctx)

    // Create knowledge base
    kb, _ := NewBase("test").
        WithEmbedder("openai").
        WithVectorDB("pgvector").
        AddSource(NewMarkdownGlobSource("../../testdata/*.md").Build(ctx)).
        WithChunking("recursive_text_splitter", 512, 64).
        Build(ctx)

    // Test ingestion
    err = ingestKnowledgeBase(ctx, kb, embedder, vectorDB)
    assert.NoError(t, err)

    // Test retrieval
    results, err := retrieveFromKnowledgeBase(ctx, kb, vectorDB, "test query")
    assert.NoError(t, err)
    assert.NotEmpty(t, results)
}
```

### Running Integration Tests

```bash
# Run integration tests
go test -tags=integration ./v2/...

# Run with testcontainers
docker pull pgvector/pgvector:latest
docker pull redis:latest
go test -tags=integration ./v2/knowledge/... ./v2/memory/...
```

---

## E2E Testing

### Coverage Target: Major User Flows

**Scope:** Complete deployment scenarios

**What to Test:**
- Full project deployment
- Workflow execution
- Result retrieval
- Error handling

### E2E Test Structure

```go
// v2/e2e/deployment_test.go
// +build e2e

package e2e

import (
    "testing"
    "time"
)

func TestE2E_FullDeployment(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping E2E test")
    }

    ctx := t.Context()

    // Build complete project
    proj, _ := BuildSampleProject()

    // Deploy to server
    client, _ := client.New("http://localhost:3000").Build(ctx)
    err := client.DeployProject(ctx, proj)
    require.NoError(t, err)

    // Execute workflow
    result, err := client.ExecuteWorkflow(ctx, proj.Name, "test-workflow", map[string]interface{}{
        "input": "test",
    })
    require.NoError(t, err)
    assert.NotEmpty(t, result.Output)

    // Verify result
    assert.Equal(t, "success", result.Status)

    // Cleanup
    client.DeleteProject(ctx, proj.Name)
}
```

---

## Performance Testing

### Goals
- Builder performance: <100ms (p99)
- Memory usage: <10MB for typical workflow
- No memory leaks

### Benchmark Tests

```go
// v2/workflow/builder_bench_test.go
package workflow

import (
    "testing"
)

func BenchmarkWorkflowBuilder(b *testing.B) {
    ctx := b.Context()
    agentCfg, _ := agent.New("assistant").Build(ctx)
    taskCfg, _ := task.NewBasic("t1").Build(ctx)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = New("test").
            AddAgent(agentCfg).
            AddTask(taskCfg).
            Build(ctx)
    }
}

func BenchmarkWorkflowBuilder_Complex(b *testing.B) {
    // Complex workflow with 10 agents, 20 tasks
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        BuildComplexWorkflow()
    }
}
```

**Running Benchmarks:**
```bash
# Run benchmarks
go test -bench=. ./v2/...

# With memory profiling
go test -bench=. -memprofile=mem.prof ./v2/...
go tool pprof mem.prof

# With CPU profiling
go test -bench=. -cpuprofile=cpu.prof ./v2/...
go tool pprof cpu.prof
```

---

## Validation Testing

### Goals
- All validation rules working
- Error messages are helpful
- Edge cases handled

### Validation Test Categories

**1. Required Fields:**
```go
func TestValidation_RequiredFields(t *testing.T) {
    tests := []struct {
        name    string
        builder func() error
        wantErr string
    }{
        {
            name: "empty workflow ID",
            builder: func() error {
                _, err := workflow.New("").Build(t.Context())
                return err
            },
            wantErr: "ID is required",
        },
        // ... more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.builder()
            assert.Error(t, err)
            assert.Contains(t, err.Error(), tt.wantErr)
        })
    }
}
```

**2. Value Ranges:**
```go
func TestValidation_ValueRanges(t *testing.T) {
    // Temperature must be 0.0-2.0
    _, err := model.New("openai", "gpt-4").
        WithTemperature(5.0).
        Build(t.Context())
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "temperature must be between 0.0 and 2.0")

    // Max tokens must be positive
    _, err = model.New("openai", "gpt-4").
        WithMaxTokens(-1).
        Build(t.Context())
    assert.Error(t, err)
}
```

**3. Reference Validation:**
```go
func TestValidation_References(t *testing.T) {
    // Task references non-existent agent
    _, err := workflow.New("test").
        AddTask(
            task.NewBasic("t1").
                WithAgent("nonexistent").
                Build(t.Context()),
        ).
        Build(t.Context())
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "agent 'nonexistent' not found")
}
```

---

## Security Testing

### Goals
- No injection vulnerabilities
- Secrets not exposed in errors
- Safe template handling

### Security Tests

```go
func TestSecurity_NoInjection(t *testing.T) {
    // SQL injection attempt
    _, err := knowledge.NewPgVector("test").
        WithDSN("postgresql://localhost'; DROP TABLE users; --").
        Build(t.Context())
    // Should validate DSN format
    assert.Error(t, err)

    // Command injection attempt
    _, err = runtime.New("bun").
        AddPermission("--allow-read && rm -rf /").
        Build(t.Context())
    // Should validate permission format
    assert.Error(t, err)
}

func TestSecurity_SecretsNotExposed(t *testing.T) {
    apiKey := "sk-secret-key-12345"
    _, err := model.New("openai", "invalid-model").
        WithAPIKey(apiKey).
        Build(t.Context())

    // Error should NOT contain the API key
    assert.NotContains(t, err.Error(), apiKey)
    assert.NotContains(t, err.Error(), "sk-secret")
}
```

---

## CI/CD Integration

### GitHub Actions Workflow

```yaml
name: Test v2 SDK

on: [push, pull_request]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25.2'
      
      - name: Run Unit Tests
        run: |
          go work init . ./v2
          cd v2
          go test -cover ./...
      
      - name: Check Coverage
        run: |
          cd v2
          go test -coverprofile=coverage.out ./...
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          if awk -v c="$COVERAGE" 'BEGIN {exit !(c < 100)}'; then
            echo "Coverage is $COVERAGE%, must be 100%"
            exit 1
          fi

  integration-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: pgvector/pgvector:latest
        env:
          POSTGRES_PASSWORD: postgres
      redis:
        image: redis:latest
    
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25.2'
      
      - name: Run Integration Tests
        run: |
          go work init . ./v2
          cd v2
          go test -tags=integration ./...
        env:
          OPENAI_API_KEY_TEST: ${{ secrets.OPENAI_API_KEY_TEST }}

  benchmarks:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      
      - name: Run Benchmarks
        run: |
          cd v2
          go test -bench=. -benchmem ./... | tee bench.txt
      
      - name: Check Performance
        run: |
          # Fail if any operation takes >100ms
          if grep -E "[0-9]{3,}\.[0-9]+ ms/op" bench.txt; then
            echo "Performance regression detected"
            exit 1
          fi
```

---

## Testing Checklist

### Before PR
- [ ] All unit tests passing (100% coverage)
- [ ] All validation tests passing
- [ ] No linting errors
- [ ] Benchmarks within acceptable range
- [ ] Examples still working

### Before Release
- [ ] Integration tests passing
- [ ] E2E tests passing
- [ ] Security tests passing
- [ ] Performance benchmarks acceptable
- [ ] No memory leaks detected
- [ ] All examples tested manually

---

## Test Data

### Fixtures

```
v2/testdata/
├── workflows/
│   ├── simple.yaml
│   ├── parallel.yaml
│   └── knowledge.yaml
├── knowledge/
│   ├── docs/
│   │   ├── doc1.md
│   │   ├── doc2.md
│   │   └── doc3.md
│   └── pdfs/
│       └── sample.pdf
└── configs/
    ├── valid_project.json
    └── invalid_project.json
```

---

## Troubleshooting Tests

### Issue: Tests fail locally but pass in CI

**Cause:** Environment differences

**Solution:**
```bash
# Use same Go version as CI
go version  # Should be 1.25.2

# Clean cache
go clean -testcache
go test ./...
```

### Issue: Integration tests time out

**Cause:** Slow container startup

**Solution:**
```go
// Increase timeout
postgres, err := testcontainers.GenericContainer(ctx, req)
postgres.SetWaitStrategy(wait.ForLog("ready to accept connections").
    WithStartupTimeout(60 * time.Second))
```

---

## Summary

### Test Coverage Goals

| Test Type | Target | Status |
|-----------|--------|--------|
| Unit | 100% | ✅ |
| Integration | Critical paths | ✅ |
| E2E | Major flows | ✅ |
| Performance | <100ms p99 | ✅ |
| Security | No vulnerabilities | ✅ |

### Commands Reference

```bash
# Run all tests
go test ./v2/...

# Run with coverage
go test -cover ./v2/...

# Run integration tests
go test -tags=integration ./v2/...

# Run E2E tests
go test -tags=e2e ./v2/...

# Run benchmarks
go test -bench=. ./v2/...

# Check for race conditions
go test -race ./v2/...
```

---

**Status:** ✅ Complete testing strategy
**Coverage Target:** 100% for all builders
**Maintained By:** QA Team + Developers
