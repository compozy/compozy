## status: completed

<task_context>
<domain>sdk/schedule</domain>
<type>implementation</type>
<scope>code_generation</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 2.0: Migrate schedule Package to Functional Options

## Overview

Migrate the `sdk/schedule` package from manual builder pattern to auto-generated functional options. The schedule package configures cron-based scheduling with interval, timezone, and jitter settings.

**Estimated Time:** 1-2 hours

<critical>
- **ALWAYS READ** @sdk/MIGRATION_GUIDE.md before starting
- **GREENFIELD APPROACH:** Build fresh in sdk/, keep sdk/ for reference
- **CRON VALIDATION:** Must validate cron expression syntax
</critical>

<requirements>
- Generate functional options from engine/schedule/config.go
- Create constructor with cron expression validation
- Validate timezone strings (use time.LoadLocation)
- Validate jitter percentages (0-100 range)
- Deep copy configuration before returning
- Comprehensive test coverage (>90%)
</requirements>

## Subtasks

- [x] 2.1 Create sdk/schedule/ directory structure
- [x] 2.2 Create generate.go with go:generate directive
- [x] 2.3 Run go generate to create options_generated.go
- [x] 2.4 Create constructor.go with cron validation
- [x] 2.5 Create constructor_test.go with schedule tests
- [x] 2.6 Verify with linter and tests
- [x] 2.7 Create README.md

## Implementation Details

### Engine Source
```go
// From engine/schedule/config.go
type Config struct {
    ID       string   // Schedule identifier
    Cron     string   // Cron expression
    Timezone string   // IANA timezone (e.g., "America/New_York")
    Jitter   int      // Jitter percentage (0-100)
}
```

### Key Validation
- Cron expression syntax (use cron parser)
- Timezone via time.LoadLocation()
- Jitter range 0-100

### Relevant Files

**Reference (for understanding):**
- `sdk/schedule/builder.go` - Old builder pattern to understand requirements
- `sdk/schedule/builder_test.go` - Old tests to understand test cases
- `engine/schedule/config.go` - Source struct for generation

**To Create in sdk/schedule/:**
- `generate.go` - go:generate directive
- `options_generated.go` - Auto-generated
- `constructor.go` - Validation logic (~70 lines)
- `constructor_test.go` - Test suite (~250 lines)
- `README.md` - API documentation

**Note:** Do NOT delete or modify anything in `sdk/schedule/` - keep for reference during transition

## Tests

- [x] Valid cron expressions (standard 5-field format)
- [x] Invalid cron expressions fail validation
- [x] Valid timezones (UTC, America/New_York, Europe/London, etc.)
- [x] Invalid timezones fail validation
- [x] Retry validation (MaxAttempts 1-100, positive backoff)
- [x] Negative retry attempts fails
- [x] Retry attempts > 100 fails
- [x] Empty ID fails
- [x] Empty cron expression fails

## Success Criteria

- [x] sdk/schedule/ directory created with proper structure
- [x] Cron expressions validated with proper error messages
- [x] Timezone validation uses time.LoadLocation
- [x] All tests pass with >90% coverage: `gotestsum -- ./sdk/schedule`
- [x] Linter passes: `golangci-lint run --fix ./sdk/schedule/...`
- [x] Code reduction: ~174 LOC → ~115 LOC (34% reduction)
