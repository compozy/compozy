## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>cli/errors</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 16.0: Comprehensive Error Handling

## Overview

Implement structured error system with consistent error codes, build contextual error messages with troubleshooting suggestions, create error recovery mechanisms for common failure scenarios, and add error logging and debugging support.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- **REUSE**: Apply existing error handling patterns with fmt.Errorf and core.NewError
- **REUSE**: Follow cli/auth/client.go error parsing and response handling patterns
- **LIBRARY**: Use golang.org/x/xerrors for error wrapping and inspection
- **REUSE**: Apply cli/auth/executor.go error display patterns for dual modes
- **REUSE**: Use logger.FromContext(ctx) for error logging and debugging
- **LIBRARY**: Use charmbracelet/lipgloss for error formatting in TUI mode
- **REUSE**: Follow existing structured error response patterns from auth module
- **LIBRARY**: Use tidwall/pretty for JSON error formatting
- Requirements: 2.5, 3.5, 4.5, 5.5, 6.5, 8.3
</requirements>

## Subtasks

- [ ] 16.1 Define structured error types and codes
- [ ] 16.2 Implement contextual error messages with suggestions
- [ ] 16.3 Create error recovery mechanisms
- [ ] 16.4 Add error logging and debugging support
- [ ] 16.5 Build error display components for TUI and JSON modes

## Implementation Details

### Structured Error System

Implement CLIError struct from techspec with Code, Message, Details, and Suggestion fields, covering all error categories.

### Contextual Messages

Create error messages that provide context about what went wrong, why it happened, and how to fix it.

### Error Recovery

Implement recovery mechanisms for common scenarios like network timeouts, configuration issues, and authentication failures.

### Error Logging

Add comprehensive error logging that helps with debugging while respecting quiet and debug modes.

### Relevant Files

- `cli/errors/types.go` - Structured error types and codes
- `cli/errors/handling.go` - Error handling utilities
- `cli/errors/display.go` - Error display for TUI and JSON
- `cli/errors/recovery.go` - Error recovery mechanisms

### Dependent Files

- `cli/shared/` - Shared utilities and constants
- `pkg/logger/` - Logging infrastructure
- All command and service files - Error handling integration

## Success Criteria

- Structured error system provides consistent error codes across all commands
- Error messages include helpful suggestions and troubleshooting guidance
- Error recovery handles common failure scenarios gracefully
- Error logging supports debugging without overwhelming users
- Error display works appropriately in both TUI and JSON modes
