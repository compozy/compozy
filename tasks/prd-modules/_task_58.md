## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>v2/compozy</domain>
<type>testing</type>
<scope>integration_tests</scope>
<complexity>medium</complexity>
<dependencies>task_56,task_57,task_01-52</dependencies>
</task_context>

# Task 58.0: Integration Tests: SDK→Engine (M)

## Overview

Create integration tests validating SDK-to-engine registration, resource validation, workflow execution, and external service integration (DB, MCP, Redis).

<critical>
- **ALWAYS READ** tasks/prd-modules/07-testing-strategy.md (integration testing section)
- **ALWAYS READ** tasks/prd-modules/_tests.md
- **ALWAYS READ** .cursor/rules/test-standards.mdc
- **MUST** use testcontainers for external dependencies
- **MUST** use t.Context() for all test contexts
- **MUST** tag integration tests: `// +build integration`
</critical>

<requirements>
- Test SDK → Engine resource registration
- Test workflow execution end-to-end
- Test knowledge system with real embedder + vectorDB
- Test memory system with Redis persistence
- Test MCP integration with real MCP server
- Test client deploy/execute/status operations
- Use testcontainers for DB, Redis
- Make tests env-gated for external APIs (OpenAI, etc.)
</requirements>

## Subtasks

- [ ] 58.1 Setup testcontainers infrastructure (Postgres+pgvector, Redis)
- [ ] 58.2 Integration test: SDK project registration → engine validation
- [ ] 58.3 Integration test: End-to-end workflow execution
- [ ] 58.4 Integration test: Knowledge system (embedder + vectorDB + retrieval)
- [ ] 58.5 Integration test: Memory system (Redis persistence + TTL)
- [ ] 58.6 Integration test: MCP integration (stdio/SSE transports)
- [ ] 58.7 Integration test: Client deploy + execute + status
- [ ] 58.8 Integration test: Hybrid SDK+YAML project
- [ ] 58.9 Create test fixtures and helper functions
- [ ] 58.10 Document integration test environment setup

## Implementation Details

**Based on:** tasks/prd-modules/07-testing-strategy.md (integration section), tasks/prd-modules/_tests.md

### Integration Test Structure

```go
// v2/integration/sdk_engine_test.go
// +build integration

package integration

import (
    "context"
    "testing"
    "os"

    "github.com/compozy/compozy/v2/internal/testutil"
    "github.com/compozy/compozy/v2/project"
    "github.com/compozy/compozy/v2/compozy"
    "github.com/testcontainers/testcontainers-go"
    "github.com/stretchr/testify/require"
)

func TestSDKToEngine_Registration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    ctx := testutil.NewTestContext(t)

    // Build SDK project
    proj, err := project.New("integration-test").
        AddModel(testutil.NewTestModel("openai", "gpt-4")).
        AddWorkflow(testutil.NewTestWorkflow("test-wf")).
        Build(ctx)
    require.NoError(t, err)

    // Initialize embedded Compozy
    app, err := compozy.New(proj).
        WithDatabase(testDB.DSN()).
        WithTemporal(testTemporal.Address()).
        WithRedis(testRedis.DSN()).
        Build(ctx)
    require.NoError(t, err)
    defer app.Shutdown(ctx)

    // Verify registration
    registered := app.ListWorkflows(ctx)
    require.Len(t, registered, 1)
    require.Equal(t, "test-wf", registered[0].ID)
}

func TestKnowledgeSystem_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Require OpenAI API key for embedder
    apiKey := os.Getenv("OPENAI_API_KEY_TEST")
    if apiKey == "" {
        t.Skip("OPENAI_API_KEY_TEST not set")
    }

    ctx := testutil.NewTestContext(t)

    // Start pgvector container
    postgres, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: testcontainers.ContainerRequest{
            Image: "pgvector/pgvector:latest",
            // ... config
        },
        Started: true,
    })
    require.NoError(t, err)
    defer postgres.Terminate(ctx)

    // Configure knowledge system (real embedder + vectorDB)
    embedder, vectorDB, kb := buildKnowledgeSystem(ctx, t, apiKey, postgres.DSN())

    // Test ingestion
    err = ingestKnowledgeBase(ctx, kb, embedder, vectorDB)
    require.NoError(t, err)

    // Test retrieval
    results, err := retrieveFromKnowledgeBase(ctx, kb, vectorDB, "test query")
    require.NoError(t, err)
    require.NotEmpty(t, results)
}
```

### Test Categories

1. **Registration Tests** (`v2/integration/registration_test.go`)
   - SDK → Engine resource registration
   - Validation propagation
   - Reference resolution

2. **Execution Tests** (`v2/integration/execution_test.go`)
   - End-to-end workflow execution
   - Task types execution
   - Signal communication

3. **Knowledge Tests** (`v2/integration/knowledge_test.go`)
   - Embedder integration (OpenAI)
   - Vector DB operations (pgvector)
   - Retrieval accuracy

4. **Memory Tests** (`v2/integration/memory_test.go`)
   - Redis persistence
   - TTL/expiration
   - Token counting

5. **MCP Tests** (`v2/integration/mcp_test.go`)
   - stdio transport
   - SSE transport
   - Tool discovery and execution

6. **Client Tests** (`v2/integration/client_test.go`)
   - Deploy project
   - Execute workflow
   - Query status

7. **Hybrid Tests** (`v2/integration/hybrid_test.go`)
   - SDK + YAML coexistence
   - AutoLoad behavior

### Relevant Files

- v2/integration/ (new directory)
- tasks/prd-modules/07-testing-strategy.md (integration patterns)
- tasks/prd-modules/_tests.md (integration test cases)

### Dependent Files

- Task 56.0 deliverable (testutil)
- Task 57.0 deliverable (unit tests as baseline)
- All builder implementations (Tasks 1-52)

## Deliverables

- `/Users/pedronauck/Dev/compozy/compozy/v2/integration/` directory with:
  - `registration_test.go` - SDK → Engine registration tests
  - `execution_test.go` - Workflow execution tests
  - `knowledge_test.go` - Knowledge system tests (pgvector + OpenAI)
  - `memory_test.go` - Memory system tests (Redis)
  - `mcp_test.go` - MCP integration tests
  - `client_test.go` - Client operations tests
  - `hybrid_test.go` - Hybrid SDK+YAML tests
  - `testdata/` - Test fixtures (YAML files, documents)
  - `helpers_test.go` - Shared test utilities
  - `README.md` - Integration test setup guide
- All tests tagged with `// +build integration`
- All tests use testcontainers for infrastructure dependencies
- Environment variable gating for external APIs

## Tests

Integration test execution:
- [ ] Run `go test -tags=integration ./v2/integration/...`
- [ ] All tests pass with testcontainers (Postgres, Redis)
- [ ] External API tests skip gracefully when env vars missing
- [ ] Tests clean up resources properly (defer cleanup)
- [ ] No test data leaks between test runs

Coverage:
- [ ] All critical SDK → Engine paths tested
- [ ] All 9 task types have execution tests
- [ ] Knowledge, memory, MCP have real integration tests
- [ ] Hybrid projects tested with actual YAML loading

Environment setup:
- [ ] README.md documents required environment variables
- [ ] README.md documents how to run with testcontainers
- [ ] CI configuration supports integration tests
- [ ] Local developer setup instructions are complete

## Success Criteria

- Integration tests validate SDK → Engine integration comprehensively
- External service integration tests use real services (not mocks)
- Testcontainers provide reproducible test environment
- Tests pass consistently in CI and locally
- Environment variable gating prevents test failures from missing keys
- Hybrid project tests validate YAML + SDK coexistence
- Integration tests catch regression in SDK → Engine contract
- Test execution time < 5 minutes with all services running
