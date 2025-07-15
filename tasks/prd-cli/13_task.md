## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>cli/output</domain>
<type>implementation</type>
<scope>middleware</scope>
<complexity>low</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 13.0: JSON Output Standardization

## Overview

Create consistent JSON response structure across all commands, implement JSON formatting with proper error handling and metadata, add JSON validation to ensure parseable output for automation tools, and build exit code management for script integration.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- **LIBRARY**: Use tidwall/pretty for fast JSON pretty-printing (5-10x faster than stdlib)
- **LIBRARY**: Use tidwall/gjson for JSON validation and parsing
- **REUSE**: Apply cli/auth/executor.go response formatting patterns
- **LIBRARY**: Use encoding/json with json.Encoder for streaming large datasets
- **REUSE**: Follow existing error handling patterns with consistent exit codes
- **LIBRARY**: Use xeipuuv/gojsonschema for output schema validation
- **REUSE**: Apply logger.FromContext(ctx) for JSON operation logging
- **REUSE**: Use cli/auth/mode.go for JSON vs TUI output selection
- Requirements: 7.3, 10.1, 10.3, 10.4
</requirements>

## Subtasks

- [ ] 13.1 Define standard JSONResponse structure and formats
- [ ] 13.2 Implement JSON output formatting utilities
- [ ] 13.3 Add metadata and pagination info to JSON responses
- [ ] 13.4 Create exit code management for script integration
- [ ] 13.5 Add JSON validation and testing utilities

## Implementation Details

### Standard JSON Structure

Implement the JSONResponse structure from techspec with Success, Data, Error, and Metadata fields, ensuring consistency across all commands.

### Output Formatting

Create JSON formatting utilities that handle different data types, pagination, and error conditions while maintaining consistent structure.

### Metadata Support

Add timestamp, request ID, and pagination information to JSON responses where appropriate.

### Exit Code Management

Implement proper exit codes for success (0), user errors (1), system errors (2), and other standard exit codes for script integration.

### Relevant Files

- `cli/formatters/json.go` - JSON output formatting utilities
- `cli/models/response.go` - Standard response structures
- `cli/output.go` - Extend output mode handling

### Dependent Files

- `cli/shared/` - Shared utilities and error types
- All command files - Integration with JSON output

## Success Criteria

- All commands produce valid, parseable JSON when --format json is specified
- JSON responses follow consistent structure with proper metadata
- Exit codes correctly indicate success/failure for script automation
- JSON output never contains ANSI color codes or TUI elements
- JSON validation ensures automation tools can reliably parse outputs
