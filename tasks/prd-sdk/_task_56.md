## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>sdk/internal</domain>
<type>testing</type>
<scope>test_infrastructure</scope>
<complexity>low</complexity>
<dependencies>task_03</dependencies>
</task_context>

# Task 56.0: Test Harness + Helpers (S)

## Overview

Create test harness infrastructure and helper functions for SDK testing: table-driven test utilities, validation test helpers, context setup, and mock fixtures.

<critical>
- **ALWAYS READ** tasks/prd-sdk/07-testing-strategy.md before starting
- **ALWAYS READ** .cursor/rules/test-standards.mdc
- **MUST** use t.Context() for all test contexts (NEVER context.Background)
- **MUST** provide helpers for common validation patterns
</critical>

<requirements>
- Create test helper package sdk/internal/testutil
- Implement context setup helpers (logger, config)
- Implement validation test helpers (error checking, BuildError inspection)
- Create table-driven test templates
- Provide mock fixtures for common resources
</requirements>

## Subtasks

- [x] 56.1 Create sdk/internal/testutil package with doc.go
- [x] 56.2 Implement context helpers (NewTestContext, WithTestLogger, WithTestConfig)
- [x] 56.3 Implement validation helpers (RequireNoError, RequireValidationError, AssertBuildError)
- [x] 56.4 Create table test template generators
- [x] 56.5 Create mock fixtures (model configs, agent configs, task configs)
- [x] 56.6 Implement comparison helpers (AssertConfigEqual)
- [x] 56.7 Add testdata organization helpers

## Implementation Details

**Based on:** tasks/prd-sdk/07-testing-strategy.md, .cursor/rules/test-standards.mdc

### Test Utilities API

```go
// sdk/internal/testutil/context.go
package testutil

import (
    "context"
    "testing"

    "github.com/compozy/compozy/pkg/config"
    "github.com/compozy/compozy/pkg/logger"
)

// NewTestContext creates a test context with logger and config
// ALWAYS uses t.Context() as base
func NewTestContext(t *testing.T) context.Context {
    t.Helper()
    ctx := t.Context()  // NEVER context.Background()
    ctx = logger.WithLogger(ctx, logger.NewTest(t))
    ctx = config.WithConfig(ctx, config.NewTest())
    return ctx
}

// sdk/internal/testutil/validation.go
package testutil

// RequireNoError fails test if err is not nil
func RequireNoError(t *testing.T, err error, msgAndArgs ...interface{})

// RequireValidationError fails test if err is nil or not a validation error
func RequireValidationError(t *testing.T, err error, contains string)

// AssertBuildError checks BuildError contains specific validation errors
func AssertBuildError(t *testing.T, err error, expectedErrors []string)

// sdk/internal/testutil/fixtures.go
package testutil

// NewTestModel creates a valid model config for testing
func NewTestModel(provider, model string) *core.ProviderConfig

// NewTestAgent creates a valid agent config for testing
func NewTestAgent(id string) *agent.Config

// NewTestWorkflow creates a minimal valid workflow for testing
func NewTestWorkflow(id string) *workflow.Config

// sdk/internal/testutil/table.go
package testutil

// TableTest represents a table-driven test case
type TableTest struct {
    Name      string
    BuildFunc func(context.Context) (interface{}, error)
    WantErr   bool
    ErrContains string
    Validate  func(*testing.T, interface{})
}

// RunTableTests executes table-driven tests
func RunTableTests(t *testing.T, tests []TableTest)
```

### Relevant Files

- sdk/internal/testutil/ (new package)
- tasks/prd-sdk/07-testing-strategy.md (testing patterns)
- .cursor/rules/test-standards.mdc (context rules)

### Dependent Files

- Task 3.0 deliverable (validation helpers package)

## Deliverables

- `/Users/pedronauck/Dev/compozy/compozy/sdk/internal/testutil/` package with:
  - `context.go` - Test context setup helpers
  - `validation.go` - Validation error helpers
  - `fixtures.go` - Mock resource configs
  - `table.go` - Table-driven test utilities
  - `compare.go` - Config comparison helpers
  - `doc.go` - Package documentation
- All helpers must use t.Helper() for proper test line reporting
- Context helpers MUST use t.Context() (not context.Background)
- Comprehensive godoc for all public functions

## Tests

Unit tests for test utilities:
- [x] TestNewTestContext verifies logger and config in context
- [x] TestNewTestContext uses t.Context() as base (not Background)
- [x] TestRequireNoError behavior with nil and non-nil errors
- [x] TestRequireValidationError detects validation errors correctly
- [x] TestAssertBuildError inspects BuildError.Errors() correctly
- [x] TestNewTestModel creates valid model config
- [x] TestNewTestAgent creates valid agent config
- [x] TestRunTableTests executes all test cases
- [x] All fixtures pass validation when built

Validation checks:
- [x] No use of context.Background() anywhere in package
- [x] All helpers call t.Helper() for proper error reporting
- [x] BuildError inspection works with sdk/internal/errors.BuildError
- [x] Fixtures match engine config structures

## Success Criteria

- Test utilities package reduces boilerplate in builder tests by 50%+
- Context setup is consistent across all SDK tests
- Validation helpers make BuildError testing straightforward
- Table test utilities support all builder test patterns
- Fixtures cover common test scenarios
- No context.Background() usage (enforced by tests)
- Documentation is clear with examples in godoc
