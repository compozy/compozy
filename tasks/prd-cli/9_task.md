## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>cli/signaling</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>temporal</dependencies>
</task_context>

# Task 9.0: Execution Signaling Functionality [⚠️ DEFERRED]

## Overview

Implement `compozy execution signal` command for sending signals to running executions, create signal payload handling for both inline and file-based input, build signal delivery confirmation and error handling, and add signal validation and target execution verification.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- **REUSE**: Apply cli/auth/executor.go CommandExecutor pattern for dual TUI/JSON modes
- **REUSE**: Extend workflow API client patterns for signal endpoints
- **LIBRARY**: Use xeipuuv/gojsonschema for signal payload validation
- **LIBRARY**: Use tidwall/gjson for payload manipulation and extraction
- **LIBRARY**: Use path/filepath.Clean for safe file path validation
- **REUSE**: Apply go-playground/validator/v10 for execution ID validation
- **REUSE**: Apply existing error handling and response formatting patterns
- **REUSE**: Use logger.FromContext(ctx) for signal operation logging
- Requirements: 6.3, 6.4, 6.5
</requirements>

## Subtasks

- [ ] 9.1 Implement `compozy execution signal <exec-id> <signal>` command
- [ ] 9.2 Add signal payload handling (--payload and --payload-file)
- [ ] 9.3 Create signal delivery confirmation and status reporting
- [ ] 9.4 Add target execution verification and validation
- [ ] 9.5 Implement error handling for signal delivery failures

## Implementation Details

### Signal Command

Implement execution signal command that can send signals to running executions, following the Signal model from the techspec.

### Payload Handling

Support both inline JSON payload via --payload flag and file-based payload via --payload-file flag, with proper validation.

### Delivery Confirmation

Provide confirmation when signals are successfully delivered, with clear error messages for delivery failures.

### Validation

Verify that target execution exists and is in a state that can receive signals before attempting delivery.

### Relevant Files

- `cli/commands/execution_signal.go` - New execution signal command
- `cli/models/signal.go` - Signal data models
- `cli/services/execution.go` - Extend execution service with signaling

### Dependent Files

- `cli/services/execution.go` - Execution service from Task 7
- `cli/shared/validation.go` - Input validation utilities
- `engine/task2/signal/` - Server-side signal handling

## Success Criteria

- `compozy execution signal <exec-id> <signal>` successfully sends signals to running executions
- Signal payload handling works for both inline and file-based input
- Signal delivery confirmation provides clear success/failure feedback
- Target execution verification prevents invalid signal attempts
- Error handling provides helpful guidance for common signal delivery issues
