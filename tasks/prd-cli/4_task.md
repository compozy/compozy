## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>cli/workflow</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 4.0: Workflow Management Foundation

## Overview

Create workflow data models and API service interfaces with interface segregation, implement workflow API client methods (list, get, execute), create workflow filtering and sorting logic, and write workflow validation and error handling utilities.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- **REUSE**: Extend cli/auth/client.go HTTP client patterns for workflow endpoints
- **REUSE**: Apply cli/auth/client.go retry logic, authentication, and JSON handling
- **LIBRARY**: Use tidwall/gjson (already in project) for JSON path queries and filtering
- **REUSE**: Follow existing error handling patterns with fmt.Errorf and core.NewError
- **REUSE**: Apply pkg/config for workflow service configuration
- **LIBRARY**: Use github.com/gorhill/cronexpr for schedule parsing and validation
- **REUSE**: Apply go-playground/validator/v10 for workflow validation
- **ENHANCED**: Implement interface segregation with separate read-only and mutate interfaces
- **ENHANCED**: Use aggressive context.Context for cancellation and timeouts
- **REUSE**: Use logger.FromContext(ctx) for API operation logging
- Requirements: 2.1, 2.2, 2.3, 2.4, 2.5
</requirements>

## Subtasks

- [ ] 4.1 Define workflow data models and types
- [ ] 4.2 Create segregated WorkflowService interfaces (reader/writer)
- [ ] 4.3 Implement workflow API client methods with context cancellation
- [ ] 4.4 Create workflow filtering and sorting utilities
- [ ] 4.5 Add workflow validation and enhanced error handling

## Implementation Details

### Data Models

Implement the Workflow, WorkflowDetail, and related types as specified in the techspec, including proper JSON tags and validation.

### Segregated Service Interfaces

Create separate interfaces for read-only operations (WorkflowReader) and mutate operations (WorkflowExecutor), enabling easier mocking in tests and potential future caching layers.

### API Client Integration

Integrate workflow methods into the unified API client from Task 1, handling authentication, error responses, timeouts, and aggressive context cancellation for long-running operations.

### Filtering and Sorting

Implement filtering by status, tags, and other criteria, plus sorting by name, creation date, and other fields with efficient JSON path queries.

### Relevant Files

- `cli/models/workflow.go` - New workflow data models
- `cli/interfaces/workflow.go` - New segregated workflow service interfaces
- `cli/services/workflow.go` - New workflow service implementation
- `cli/api_client.go` - Extend with workflow methods and context handling

### Dependent Files

- `cli/api_client.go` - API client foundation from Task 1
- `engine/workflow/router/` - Server-side workflow APIs
- `cli/shared/` - Shared utilities from Task 1

## Success Criteria

- Workflow data models properly represent server API responses with validation
- Segregated WorkflowService interfaces enable easier testing and future extensions
- API client methods handle all workflow operations with proper context cancellation
- Filtering and sorting work efficiently for large workflow collections using JSON path queries
- Error handling provides helpful feedback with enhanced context for common failure scenarios
