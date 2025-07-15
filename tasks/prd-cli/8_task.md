## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>cli/commands</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>temporal</dependencies>
</task_context>

# Task 8.0: Execution Management Commands [⚠️ DEFERRED]

## Overview

Create execution listing command with filtering by workflow and status, implement `compozy execution get` command with detailed execution information, build execution log display with optional log inclusion, and add execution search and filtering capabilities.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- **REUSE**: Apply cli/auth/executor.go CommandExecutor pattern for dual TUI/JSON modes
- **REUSE**: Extend workflow API client patterns for execution endpoints
- **LIBRARY**: Use charmbracelet/bubbles/table for execution listing with sorting
- **LIBRARY**: Use tidwall/gjson for execution filtering and search capabilities
- **LIBRARY**: Use tidwall/pretty for JSON output formatting
- **REUSE**: Apply existing pagination patterns from auth module
- **REUSE**: Use cli/auth/mode.go DetectMode() for output selection
- **REUSE**: Apply logger.FromContext(ctx) for operation logging
- Requirements: 4.1, 4.2, 4.3, 4.4, 4.5
</requirements>

## Subtasks

- [ ] 8.1 Implement `compozy execution list` command
- [ ] 8.2 Create `compozy execution get <exec-id>` command
- [ ] 8.3 Add filtering by workflow ID and execution status
- [ ] 8.4 Build execution log display with --show-logs flag
- [ ] 8.5 Implement execution search and advanced filtering

## Implementation Details

### Execution List Command

Create execution listing with table display showing execution ID, workflow, status, start time, duration, and basic metrics.

### Execution Detail Command

Implement detailed execution view showing logs, task results, metrics, and full execution context as specified in ExecutionDetail model.

### Filtering System

Add filtering by workflow ID (--workflow), status (--status), date ranges, and other criteria for both TUI and JSON modes.

### Log Display

Implement optional log inclusion with --show-logs flag, with proper formatting and scrolling capabilities.

### Relevant Files

- `cli/commands/execution_list.go` - New execution list command
- `cli/commands/execution_get.go` - New execution detail command
- `cli/models/execution.go` - Execution data models
- `cli/tui/components/execution_table.go` - Execution table component

### Dependent Files

- `cli/services/execution.go` - Execution service from Task 7
- `cli/tui/components/` - Existing TUI table components
- `engine/workflow/activities/` - Server-side execution APIs

## Success Criteria

- `compozy execution list` displays executions in filterable table format
- Filtering by workflow and status works correctly in both TUI and JSON modes
- `compozy execution get <exec-id>` shows comprehensive execution information
- Log display with --show-logs provides readable, scrollable output
- Search and filtering handle large execution collections efficiently
