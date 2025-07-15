## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>cli/commands</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 6.0: Workflow Detail and Execution Commands

## Overview

Implement `compozy workflow get` command with detailed workflow information, create workflow execution command with input parameter handling, build execution result display for both TUI and JSON modes, and add input validation for workflow execution parameters.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- **REUSE**: Apply cli/auth/executor.go CommandExecutor pattern for dual TUI/JSON modes
- **REUSE**: Extend workflow API client from task 4 for get and execute endpoints
- **LIBRARY**: Use charmbracelet/bubbles/textinput for parameter input forms
- **REUSE**: Apply go-playground/validator/v10 for input parameter validation
- **LIBRARY**: Use tidwall/gjson for parameter extraction and manipulation
- **LIBRARY**: Use xeipuuv/gojsonschema for JSON schema validation of execution payloads
- **REUSE**: Apply existing error handling and response formatting patterns
- **REUSE**: Use logger.FromContext(ctx) for execution logging
- Requirements: 2.3, 2.4, 3.1, 3.3, 3.4
</requirements>

## Subtasks

- [ ] 6.1 Implement `compozy workflow get <id>` command
- [ ] 6.2 Create `compozy workflow execute <id>` command
- [ ] 6.3 Add input parameter handling (--input and --input-file)
- [ ] 6.4 Build execution result display components
- [ ] 6.5 Implement input validation and schema checking

## Implementation Details

### Workflow Detail Command

Create detailed workflow view showing components, tasks, inputs, outputs, schedule, and statistics as specified in WorkflowDetail model from techspec.

### Execution Command

Implement workflow execution with support for inline input via --input flag and file-based input via --input-file flag.

### Input Handling

Support both JSON input parameters and file-based input, with proper validation against workflow input schemas.

### Result Display

Create components to display execution results in both TUI (styled, interactive) and JSON (machine-readable) formats.

### Relevant Files

- `cli/commands/workflow_get.go` - New workflow detail command
- `cli/commands/workflow_execute.go` - New workflow execution command
- `cli/tui/components/workflow_detail.go` - Workflow detail display component
- `cli/tui/components/execution_result.go` - Execution result display component

### Dependent Files

- `cli/services/workflow.go` - Workflow service from Task 4
- `cli/shared/validation.go` - Input validation utilities
- `cli/formatters/` - Output formatting from previous tasks

## Success Criteria

- `compozy workflow get <id>` shows comprehensive workflow information
- `compozy workflow execute <id>` starts execution and returns execution ID
- Input validation prevents invalid parameters and provides helpful error messages
- Execution results display properly in both TUI and JSON modes
- File-based input works correctly with proper error handling for file operations
