## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>pkg/config|cli/cmd</domain>
<type>implementation</type>
<scope>configuration|cli</scope>
<complexity>low</complexity>
<dependencies>config_loader|cli</dependencies>
</task_context>

# Task 11.0: Configuration Validation & CLI [Size: S - ≤ half-day]

## Overview

Add validation rules for mode configuration and update CLI commands to support the new mode configuration. This includes validation in the config loader, updates to CLI flags, and enhancements to config display and diagnostics commands.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
- **MUST** use `config.FromContext(ctx)` - never store config
- **MUST** use `logger.FromContext(ctx)` - never pass logger as parameter
</critical>

<research>
# When you need information about a library or external API:
- use perplexity and context7 to find out how to properly fix/resolve this
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know, you don't need to pass a query-style search phrase, the same for the topic param of context7
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</research>

<requirements>
- Mode field must be validated (standalone | distributed | empty)
- Component mode fields must be validated
- Invalid mode configurations must be rejected with clear error messages
- CLI `--mode` flag must be added to `compozy start`
- `compozy config show` must display mode configuration
- `compozy config diagnostics` must show effective mode resolution
- All validation errors must be helpful and actionable
</requirements>

## Subtasks

- [x] 11.1 Add mode validation rules to config loader
- [x] 11.2 Add `--mode` flag to `compozy start` command
- [x] 11.3 Update `compozy config show` to display mode configuration
- [x] 11.4 Update `compozy config diagnostics` to show mode resolution
- [x] 11.5 Add CLI tests with golden files
- [x] 11.6 Add validation error message tests

## Implementation Details

### Validation Rules

Add validation to `pkg/config/loader.go` that checks:
- Global `mode` field is empty or one of: "standalone", "distributed"
- Component `mode` fields are empty or one of: "standalone", "distributed"
- Redis standalone persistence config is valid when enabled
- Mode-specific requirements are met (e.g., Redis address when distributed)

### CLI Updates

Update CLI commands to support mode configuration:
- Add `--mode` flag to `compozy start` command
- Display mode in config show/diagnostics output
- Show effective mode resolution for each component

### Relevant Files

**Files to Update:**
- `pkg/config/loader.go` - Add mode validation rules
- `cli/cmd/start/start.go` - Add `--mode` flag
- `cli/cmd/config/show.go` - Display mode configuration
- `cli/cmd/config/diagnostics.go` - Show mode resolution

**Files to Create:**
- `cli/cmd/config/config_test.go` - CLI tests with goldens
- `testdata/config-show-standalone.golden` - Expected output for standalone config
- `testdata/config-show-mixed.golden` - Expected output for mixed mode config

### Dependent Files

- `pkg/config/config.go` - Config structs (Task 1.0)
- `pkg/config/resolver.go` - Mode resolution logic (Task 1.0)

## Deliverables

- Mode validation rules in config loader with clear error messages
- `--mode` flag added to `compozy start` command
- `compozy config show` displays global and component modes
- `compozy config diagnostics` shows effective mode resolution for all components
- CLI tests with golden files for mode-related output
- Validation error tests for invalid mode configurations
- Help text and documentation for CLI flags
- All changes passing `make lint` and `make test`

## Tests

Unit tests mapped from `_tests.md` for this feature:

### Configuration Validation Tests (`pkg/config/loader_test.go`)

- [x] Should validate global mode field (standalone | distributed | empty)
- [x] Should validate component mode fields
- [x] Should reject invalid mode values with clear error message
- [x] Should allow empty mode values (inheritance)
- [x] Should validate Redis persistence configuration when enabled
- [x] Should validate mode-specific requirements (Redis addr when distributed)
- [x] Should validate snapshot interval is positive duration
- [x] Should validate data directory path is valid
- [x] Should accept valid standalone configurations
- [x] Should accept valid distributed configurations
- [x] Should accept valid mixed mode configurations

### CLI Flag Tests (`cli/cmd/start/start_test.go`)

- [x] Should accept `--mode standalone` flag
- [x] Should accept `--mode distributed` flag
- [x] Should reject invalid `--mode` values
- [x] Should prioritize config file over CLI flags
- [x] Should merge CLI flags with config file correctly
- [x] Should display mode in startup logs

### Config Show Tests (`cli/cmd/config/config_test.go`)

- [x] Should show global mode in output
- [x] Should show component modes in output
- [x] Should show Redis standalone persistence config
- [x] Should format mode configuration clearly
- [x] Should match golden file for standalone config
- [x] Should match golden file for mixed mode config

### Config Diagnostics Tests (`cli/cmd/config/config_test.go`)

- [x] Should display effective mode resolution for Redis
- [x] Should display effective mode resolution for Temporal
- [x] Should display effective mode resolution for MCPProxy
- [x] Should show mode inheritance clearly
- [x] Should highlight mode overrides
- [x] Should show default fallback mode

### Error Message Tests

- [x] Should provide helpful error for invalid global mode
- [x] Should provide helpful error for invalid component mode
- [x] Should provide helpful error for missing Redis address in distributed mode
- [x] Should provide helpful error for invalid persistence config
- [x] Should provide helpful error for invalid snapshot interval

### Golden File Tests

- [x] `testdata/config-show-standalone.golden` - Standalone config output
- [x] `testdata/config-show-mixed.golden` - Mixed mode config output
- [x] `testdata/config-diagnostics-standalone.golden` - Diagnostics output
- [x] Golden files should be regenerated with `--update-golden` flag

## Success Criteria

- All validation rules implemented and working correctly
- Invalid mode configurations are rejected with clear, actionable error messages
- `--mode` flag works correctly in `compozy start` command
- Config file mode takes precedence over CLI flag
- `compozy config show` displays all mode configuration clearly
- `compozy config diagnostics` shows effective mode resolution for all components
- CLI tests with golden files pass
- Golden files accurately represent expected output
- All validation tests pass with `make test`
- All lint checks pass with `make lint`
- Help text is clear and accurate
- Error messages are helpful and guide users to fixes
