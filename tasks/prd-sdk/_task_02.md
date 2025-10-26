## status: completed

<task_context>
<domain>sdk/internal/errors</domain>
<type>implementation</type>
<scope>core_infrastructure</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 02.0: Error Aggregation Infra (S)

## Overview

Implement BuildError infrastructure for accumulating multiple builder errors and reporting them together. This enables fluent API patterns where errors are collected during builder method calls and reported at Build() time.

<critical>
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md (Error Handling Strategy section)
- **MUST** support errors.Is and errors.As for compatibility
- **MUST** provide clear, actionable error messages
</critical>

<requirements>
- Create BuildError type that aggregates multiple errors
- Implement Error() method with clear formatting
- Implement Unwrap() for errors.Is/As compatibility
- Support single error and multiple error cases
- Provide numbered error list for multiple errors
</requirements>

## Subtasks

- [x] 02.1 Create sdk/internal/errors/build_error.go
- [x] 02.2 Implement BuildError struct with Errors []error field
- [x] 02.3 Implement Error() method with formatted output
- [x] 02.4 Implement Unwrap() method for compatibility
- [x] 02.5 Add unit tests for all error aggregation scenarios
- [x] 02.6 Test errors.Is and errors.As integration

## Implementation Details

Reference: tasks/prd-sdk/02-architecture.md (Error Handling Strategy)

### BuildError Type

```go
// sdk/internal/errors/build_error.go
package errors

type BuildError struct {
    Errors []error
}

func (e *BuildError) Error() string
func (e *BuildError) Unwrap() error
```

### Error Formatting

- Single error: "build failed: <error>"
- Multiple errors: "build failed with N errors:\n  1. <error1>\n  2. <error2>..."

### Relevant Files

- `sdk/internal/errors/build_error.go` (NEW)
- `sdk/internal/errors/build_error_test.go` (NEW)

### Dependent Files

- None (foundation for all builders)

## Deliverables

- ✅ `sdk/internal/errors/build_error.go` with BuildError type
- ✅ Error() method with clear formatting
- ✅ Unwrap() method for errors.Is/As compatibility
- ✅ Unit tests with 95%+ coverage
- ✅ Examples in tests showing usage pattern

## Tests

Reference: tasks/prd-sdk/_tests.md

- Unit tests for BuildError:
- [x] Test single error formatting
- [x] Test multiple errors formatting
- [x] Test empty errors list
- [x] Test errors.Is integration
- [x] Test errors.As integration
- [x] Test Unwrap returns first error
- [x] Test error message clarity and readability

## Success Criteria

- BuildError aggregates multiple errors correctly
- Error messages are clear and numbered for multiple errors
- errors.Is and errors.As work correctly
- All tests pass with 95%+ coverage
- Error output is actionable for developers
