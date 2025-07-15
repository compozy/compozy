## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>cli/events</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 11.0: Event Sending Functionality [⚠️ DEFERRED]

## Overview

Create `compozy event send` command for triggering workflow events, implement event payload handling for both inline and file-based input, build event delivery confirmation and error reporting, and add event validation and workflow trigger verification.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- **REUSE**: Apply cli/auth/executor.go CommandExecutor pattern for dual TUI/JSON modes
- **REUSE**: Extend workflow API client patterns for event endpoints
- **LIBRARY**: Use xeipuuv/gojsonschema for event payload validation
- **LIBRARY**: Use tidwall/gjson for payload manipulation and extraction
- **LIBRARY**: Use path/filepath.Clean for safe file path validation
- **REUSE**: Apply go-playground/validator/v10 for workflow ID validation
- **REUSE**: Apply existing error handling and response formatting patterns
- **REUSE**: Use logger.FromContext(ctx) for event operation logging
- Requirements: 6.1, 6.2, 6.4
</requirements>

## Subtasks

- [ ] 11.1 Implement `compozy event send <event-name>` command
- [ ] 11.2 Add event payload handling (--payload and --payload-file)
- [ ] 11.3 Create event delivery confirmation and status reporting
- [ ] 11.4 Add event validation and workflow trigger verification
- [ ] 11.5 Implement error handling for event delivery failures

## Implementation Details

### Event Command

Implement event send command that can trigger workflow events, following the Event model from the techspec.

### Payload Handling

Support both inline JSON payload via --payload flag and file-based payload via --payload-file flag, with proper validation.

### Delivery Confirmation

Provide confirmation when events are successfully delivered, showing which workflows were triggered.

### Validation

Validate event names and payloads, and verify that events will trigger expected workflows.

### Relevant Files

- `cli/commands/event_send.go` - New event send command
- `cli/models/event.go` - Event data models
- `cli/services/event.go` - New event service

### Dependent Files

- `cli/shared/validation.go` - Input validation utilities
- `engine/workflow/router/` - Server-side event handling
- `cli/formatters/` - Output formatting utilities

## Success Criteria

- `compozy event send <event-name>` successfully triggers workflow events
- Event payload handling works for both inline and file-based input
- Event delivery confirmation shows which workflows were triggered
- Event validation prevents invalid event names and malformed payloads
- Error handling provides helpful guidance for event delivery issues
