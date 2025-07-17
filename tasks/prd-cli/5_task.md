## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>cli/commands</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 5.0: Workflow Listing Functionality

## Overview

Create workflow list command with dual TUI/JSON output modes, build interactive workflow table component with sorting and filtering, implement JSON output formatter for workflow lists, and add pagination support for large workflow collections.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- **REUSE**: Apply cli/auth/executor.go CommandExecutor pattern for dual TUI/JSON modes
- **REUSE**: Extend cli/tui patterns for workflow table component (avoid tight coupling from ListModel)
- **LIBRARY**: Use charmbracelet/bubbles/table for interactive sorting and filtering
- **LIBRARY**: Use tidwall/pretty for fast JSON pretty-printing (5-10x faster than stdlib)
- **LIBRARY**: Use tidwall/gjson for filtering and search capabilities
- **REUSE**: Apply existing pagination patterns from auth module if available
- **REUSE**: Use cli/auth/mode.go DetectMode() for output selection
- **REUSE**: Apply logger.FromContext(ctx) for operation logging
- Requirements: 2.1, 2.2, 10.1, 10.2
</requirements>

## Subtasks

- [x] 5.1 Implement `compozy workflow list` command
- [x] 5.2 Create interactive workflow table TUI component
- [x] 5.3 Build JSON output formatter for workflow lists
- [x] 5.4 Add pagination support for large collections
- [x] 5.5 Implement filtering and sorting in TUI mode

## Implementation Details

### Command Implementation

Create the workflow list command following the dual-handler pattern from Task 1, supporting both TUI and JSON output modes.

### Interactive Table Component

Build an enhanced table component based on existing TUI components, with sorting, filtering, and navigation capabilities for workflow data.

### JSON Output

Implement consistent JSON output formatting following the JSONResponse structure from the techspec, with proper metadata and pagination info.

### Pagination Support

Add pagination for both TUI and JSON modes to handle large workflow collections efficiently.

### Relevant Files

- `cli/commands/workflow_list.go` - New workflow list command
- `cli/tui/components/workflow_table.go` - New workflow table component
- `cli/formatters/json.go` - JSON output formatting utilities

### Dependent Files

- `cli/services/workflow.go` - Workflow service from Task 4
- `cli/tui/components/` - Existing TUI table components
- `cli/shared/` - Shared utilities from Task 1

## Success Criteria

- `compozy workflow list` displays workflows in styled table format in TUI mode
- JSON output mode produces machine-readable format for automation
- Interactive table supports sorting by name, status, creation date
- Filtering works by workflow status, tags, and search terms
- Pagination handles large workflow collections efficiently
