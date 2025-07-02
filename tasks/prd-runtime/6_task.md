---
status: completed # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/worker</domain>
<type>integration</type>
<scope>middleware</scope>
<complexity>medium</complexity>
<dependencies>temporal</dependencies>
</task_context>

# Task 6.0: Update Worker Integration

## Overview

Update the worker and task execution layers to use the new runtime interface. This ensures all tool executions throughout the system use the new architecture.

## Subtasks

- [x] 6.1 Update `engine/worker/mod.go` to use runtime interface
- [x] 6.2 Modify `engine/task/activities/exec_basic.go` for new runtime
- [x] 6.3 Update `engine/llm/tool.go` integration points
- [x] 6.4 Ensure proper context and timeout propagation
- [x] 6.5 Update error handling for new runtime errors
- [x] 6.6 Write integration tests for all execution paths

## Implementation Details

### Worker Integration Points

- Replace direct RuntimeManager usage with interface
- Use factory pattern to create appropriate runtime
- Maintain existing error handling patterns
- Preserve timeout and context behavior

### Task Activity Updates

```go
// Update exec_basic.go to use runtime interface
runtime, err := runtimeFactory.Create(ctx, projectRoot, config)
if err != nil {
    return nil, err
}

output, err := runtime.ExecuteToolWithTimeout(ctx, toolID, input, env, timeout)
```

### LLM Tool Integration

- Update tool execution to use new runtime
- Maintain tool registry compatibility
- Preserve LLM tool calling interface
- Handle runtime-specific errors appropriately

## Success Criteria

- All tool executions use new runtime interface
- No breaking changes to existing workflows
- Proper error propagation maintained
- Timeouts and contexts work correctly
- Integration tests pass for all scenarios
- Performance characteristics preserved or improved

<critical>
**MANDATORY REQUIREMENTS:**

- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test` before completing parent tasks
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks

**Enforcement:** Violating these standards results in immediate task rejection.
</critical>
