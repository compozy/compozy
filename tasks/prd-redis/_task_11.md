## markdown

## status: pending # Options: pending, in-progress, completed, excluded

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

- [ ] 11.1 Add mode validation rules to config loader
- [ ] 11.2 Add `--mode` flag to `compozy start` command
- [ ] 11.3 Update `compozy config show` to display mode configuration
- [ ] 11.4 Update `compozy config diagnostics` to show mode resolution
- [ ] 11.5 Add CLI tests with golden files
- [ ] 11.6 Add validation error message tests

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

- [ ] Should validate global mode field (standalone | distributed | empty)
- [ ] Should validate component mode fields
- [ ] Should reject invalid mode values with clear error message
- [ ] Should allow empty mode values (inheritance)
- [ ] Should validate Redis persistence configuration when enabled
- [ ] Should validate mode-specific requirements (Redis addr when distributed)
- [ ] Should validate snapshot interval is positive duration
- [ ] Should validate data directory path is valid
- [ ] Should accept valid standalone configurations
- [ ] Should accept valid distributed configurations
- [ ] Should accept valid mixed mode configurations

### CLI Flag Tests (`cli/cmd/start/start_test.go`)

- [ ] Should accept `--mode standalone` flag
- [ ] Should accept `--mode distributed` flag
- [ ] Should reject invalid `--mode` values
- [ ] Should prioritize config file over CLI flags
- [ ] Should merge CLI flags with config file correctly
- [ ] Should display mode in startup logs

### Config Show Tests (`cli/cmd/config/config_test.go`)

- [ ] Should show global mode in output
- [ ] Should show component modes in output
- [ ] Should show Redis standalone persistence config
- [ ] Should format mode configuration clearly
- [ ] Should match golden file for standalone config
- [ ] Should match golden file for mixed mode config

### Config Diagnostics Tests (`cli/cmd/config/config_test.go`)

- [ ] Should display effective mode resolution for Redis
- [ ] Should display effective mode resolution for Temporal
- [ ] Should display effective mode resolution for MCPProxy
- [ ] Should show mode inheritance clearly
- [ ] Should highlight mode overrides
- [ ] Should show default fallback mode

### Error Message Tests

- [ ] Should provide helpful error for invalid global mode
- [ ] Should provide helpful error for invalid component mode
- [ ] Should provide helpful error for missing Redis address in distributed mode
- [ ] Should provide helpful error for invalid persistence config
- [ ] Should provide helpful error for invalid snapshot interval

### Golden File Tests

- [ ] `testdata/config-show-standalone.golden` - Standalone config output
- [ ] `testdata/config-show-mixed.golden` - Mixed mode config output
- [ ] `testdata/config-diagnostics-standalone.golden` - Diagnostics output
- [ ] Golden files should be regenerated with `--update-golden` flag

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
