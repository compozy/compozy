## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>cli/monitoring</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>temporal</dependencies>
</task_context>

# Task 7.0: Real-time Execution Monitoring [⚠️ DEFERRED]

## Overview

Implement execution following with `--follow` flag and real-time progress display, create TUI progress monitor component with live log streaming, build execution event handling and display system with aggressive context cancellation, and add execution status tracking and completion notifications.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- **LIBRARY**: Use github.com/advbet/sseclient for real-time execution updates via SSE
- **LIBRARY**: Use github.com/gorilla/websocket for bidirectional execution communication
- **LIBRARY**: Use github.com/schollz/progressbar/v3 for thread-safe progress tracking
- **LIBRARY**: Use charmbracelet/bubbletea for real-time TUI updates with live streaming
- **REUSE**: Apply cli/auth/executor.go pattern with extended handlers for follow mode
- **LIBRARY**: Use tidwall/gjson for parsing execution events and log filtering
- **REUSE**: Use cli/auth/mode.go for follow vs JSON output mode selection
- **ENHANCED**: Implement aggressive context.Context usage for cancellation and timeouts
- **ENHANCED**: Add proper context propagation through streaming connections and goroutines
- **REUSE**: Apply logger.FromContext(ctx) for monitoring operation logging
- Requirements: 3.2, 3.4, 3.5
</requirements>

## Subtasks

- [ ] 7.1 Implement `--follow` flag for workflow execution with context cancellation
- [ ] 7.2 Create real-time progress monitoring TUI component with context handling
- [ ] 7.3 Build execution event streaming with proper context propagation
- [ ] 7.4 Add live log display with scrolling, filtering, and cancellation support
- [ ] 7.5 Implement completion notifications with graceful context cleanup

## Implementation Details

### Follow Mode Implementation

Extend workflow execution command with --follow flag that switches to real-time monitoring mode instead of just returning execution ID. Implement aggressive context cancellation to ensure streaming connections are properly closed on interruption.

### Progress Monitor Component

Create ProgressMonitor TUI component as specified in techspec, with real-time updates, log streaming, and status display. Ensure all goroutines respect context cancellation for clean shutdown.

### Event Streaming with Context Propagation

Implement execution event streaming using WebSocket or Server-Sent Events for real-time updates from the server. Propagate context through all streaming connections and ensure proper cleanup on cancellation.

### Log Display with Cancellation

Create scrollable log display with filtering capabilities, timestamps, and proper formatting for different log levels. All background operations must respect context cancellation.

### Relevant Files

- `cli/commands/workflow_execute.go` - Extend with --follow flag and context handling
- `cli/tui/components/progress_monitor.go` - New progress monitoring component with context
- `cli/services/execution.go` - New execution service for streaming with context propagation
- `cli/streaming/` - New streaming utilities with context cancellation support

### Dependent Files

- `cli/services/workflow.go` - Workflow service from Task 4
- `cli/tui/components/` - Existing TUI components
- `engine/task/activities/` - Server-side execution tracking

## Success Criteria

- `--follow` flag provides real-time execution monitoring with proper context cancellation
- Progress monitor shows current task, overall progress, and live logs with graceful interruption handling
- Event streaming handles network interruptions and context cancellation gracefully
- Log display allows scrolling through execution history with responsive cancellation
- Completion notifications clearly indicate success/failure with proper context cleanup and resource management
