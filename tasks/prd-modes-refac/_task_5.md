## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine|pkg|cli</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>config|cache|temporal|server</dependencies>
</task_context>

# Task 5.0: Standardize Comments & Log Messages

## Overview

This task standardizes all comments, log messages, and CLI help text across the codebase to replace "standalone" terminology with "embedded" or specific mode names. This ensures consistency in documentation and user-facing messages.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the _techspec.md and the _prd.md docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</critical>

<research>
# When you need information about a library or external API:
- use perplexity and context7 to find out how to properly fix/resolve this
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know, you don't need to pass a query-style search phrase, the same for the topic param of context7
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</research>

<requirements>
- Replace "standalone" in comments with "embedded" or specific mode names
- Update log messages to use actual mode values instead of "standalone"
- Update CLI help text to reflect new mode terminology
- Use grep to find all occurrences: `grep -r "standalone" --include="*.go" pkg/ engine/ cli/`
- Standardize terminology: use "embedded" for memory/persistent modes collectively
</requirements>

## Subtasks

- [x] 5.1 Search for all "standalone" occurrences in Go files: `grep -r "standalone" --include="*.go" pkg/ engine/ cli/`
- [x] 5.2 Update comments in `engine/` directory
- [x] 5.3 Update comments in `pkg/` directory
- [x] 5.4 Update comments in `cli/` directory
- [x] 5.5 Update log messages to use actual mode values
- [x] 5.6 Update function documentation comments
- [x] 5.7 Update struct field documentation
- [x] 5.8 Update CLI help text in `cli/cmd/start/start.go`
- [x] 5.9 Update CLI help text in `cli/helpers/mode.go` (if exists)
- [x] 5.10 Review and update any other CLI command files mentioning modes
- [x] 5.11 Verify grep results show no inappropriate "standalone" usage (except validation errors)

## Implementation Details

See Phase 3.4 and Phase 3.5 in the techspec for detailed implementation steps.

Key changes:
- Replace "standalone" in comments with "embedded" or specific mode names
- Update log messages to use actual mode values: `log.Info("mode", mode)` instead of `log.Info("standalone mode")`
- Update CLI help text to show memory/persistent/distributed modes
- Use grep to systematically find and replace all occurrences

Example transformations:
- `// Start standalone Temporal server` → `// Start embedded Temporal server for memory/persistent modes`
- `log.Info("Starting standalone server")` → `log.Info("Starting embedded Temporal", "mode", mode)`

### Relevant Files

- All `.go` files in `engine/` directory
- All `.go` files in `pkg/` directory
- All `.go` files in `cli/` directory
- `cli/cmd/start/start.go`
- `cli/helpers/mode.go` (if exists)

### Dependent Files

- All files updated in previous tasks (1.0, 2.0, 3.0, 4.0)

## Deliverables

- All comments updated to use "embedded" terminology
- All log messages updated to use actual mode values
- CLI help text updated to reflect new modes
- No inappropriate "standalone" references (except validation errors)
- Code documentation consistent
- User-facing messages accurate

## Tests

- Unit tests:
- [x] Verify log messages output correct mode values
- [x] Verify CLI help text shows correct modes

- Integration tests:
- [x] Verify server startup logs use correct terminology
- [x] Verify CLI commands display correct mode information

- Manual verification:
- [x] Run grep to verify no inappropriate "standalone" usage remains
- [x] Review CLI help output
- [x] Review log output during server startup

## Success Criteria

- All comments use "embedded" or specific mode names
- All log messages use actual mode values
- CLI help text updated correctly
- No grep results for inappropriate "standalone" usage (except validation errors and test names that will be updated)
- All tests pass (`make test`)
- Linter passes (`make lint`)
- Code documentation is consistent and clear
- User-facing messages are accurate
