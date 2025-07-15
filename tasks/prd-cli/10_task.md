## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>cli/schedule</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>temporal</dependencies>
</task_context>

# Task 10.0: Schedule Management System

## Overview

Create schedule data models and API service interfaces, implement schedule listing command with CRON expression display, build schedule update command with CRON validation, and create schedule deletion command with confirmation prompts.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- **REUSE**: Apply cli/auth/executor.go CommandExecutor pattern for dual TUI/JSON modes
- **REUSE**: Extend workflow API client patterns for schedule endpoints
- **LIBRARY**: Use github.com/gorhill/cronexpr for CRON parsing and validation
- **LIBRARY**: Use charmbracelet/bubbles/table for schedule listing with sorting
- **LIBRARY**: Use charmbracelet/bubbles/textinput for CRON expression input
- **REUSE**: Apply go-playground/validator/v10 for schedule validation
- **LIBRARY**: Use tidwall/pretty for JSON output formatting
- **REUSE**: Apply logger.FromContext(ctx) for schedule operation logging
- Requirements: 5.1, 5.2, 5.3, 5.4, 5.5
</requirements>

## Subtasks

- [ ] 10.1 Define schedule data models and types
- [ ] 10.2 Implement `compozy schedule list` command
- [ ] 10.3 Create `compozy schedule update <workflow-id>` command
- [ ] 10.4 Build `compozy schedule delete <workflow-id>` command
- [ ] 10.5 Add CRON expression validation and help

## Implementation Details

### Schedule Models

Implement Schedule model as specified in techspec, with WorkflowID, CronExpr, Enabled, NextRun, LastRun, and Timezone fields.

### Schedule Listing

Create schedule list command showing all scheduled workflows with their CRON expressions, next run times, and enabled status.

### Schedule Updates

Implement schedule update command with --cron flag for setting CRON expressions and --enabled flag for enabling/disabling schedules.

### Schedule Deletion

Create schedule deletion command with confirmation prompts to prevent accidental deletions.

### CRON Validation

Add comprehensive CRON expression validation with helpful error messages and examples for common patterns.

### Relevant Files

- `cli/commands/schedule_list.go` - New schedule list command
- `cli/commands/schedule_update.go` - New schedule update command
- `cli/commands/schedule_delete.go` - New schedule delete command
- `cli/models/schedule.go` - Schedule data models

### Dependent Files

- `cli/services/workflow.go` - Workflow service integration
- `engine/workflow/schedule/` - Server-side schedule management
- `cli/shared/validation.go` - CRON validation utilities

## Success Criteria

- `compozy schedule list` displays all scheduled workflows with CRON expressions
- `compozy schedule update` properly validates CRON expressions and updates schedules
- `compozy schedule delete` includes confirmation prompts to prevent accidents
- CRON validation provides helpful error messages and examples
- All schedule commands work in both TUI and JSON output modes
