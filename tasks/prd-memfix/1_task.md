---
status: completed # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/task2</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 1.0: Core Factory Implementation

## Overview

Implement the missing Task2 factory cases to support memory task normalizer and response handler creation. This is the critical fix that unblocks the entire memory task functionality.

<critical>
**MANDATORY REQUIREMENTS:**
- **ALWAYS** check dependent files APIs before write tests to avoid write wrong code
- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test` before completing ANY subtask
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks
**Enforcement:** Violating these standards results in immediate task rejection.
</critical>

## Subtasks

- [x] 1.1 Add memory task case to CreateNormalizer method in factory.go ✅
- [x] 1.2 Add memory task case to CreateResponseHandler method in factory.go ✅
- [x] 1.3 Verify factory methods return appropriate instances without errors ✅
- [x] 1.4 Run linter and ensure code follows project standards ✅

## Implementation Details

### Technical Requirements

The Task2 factory (`engine/task2/factory.go`) requires two simple additions:

1. **CreateNormalizer Method** (around line 81):
    - Add `case task.TaskTypeMemory:` after `case task.TaskTypeSignal:`
    - Return `basic.NewNormalizer(f.templateEngine), nil`
    - This reuses the basic normalizer which is sufficient for memory task needs

2. **CreateResponseHandler Method** (around line 151):
    - Add `case task.TaskTypeMemory:` after `case task.TaskTypeAggregate:`
    - Return `basic.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil`
    - This reuses the basic response handler for standard output processing

### Code Changes

**File: `engine/task2/factory.go`**

```go
// In CreateNormalizer method, add after line 81:
case task.TaskTypeMemory:
    return basic.NewNormalizer(f.templateEngine), nil

// In CreateResponseHandler method, add after line 151:
case task.TaskTypeMemory:
    return basic.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
```

### Relevant Files

> Files that this task will modify:

- `engine/task2/factory.go` - The main file requiring changes

### Dependent Files

> Files that must be checked for compatibility:

- `engine/task2/basic/normalizer.go` - Basic normalizer being reused
- `engine/task2/basic/response_handler.go` - Basic response handler being reused
- `engine/task/config.go` - Contains TaskTypeMemory constant definition

## Success Criteria

- [x] Factory CreateNormalizer method returns a normalizer for TaskTypeMemory without error ✅
- [x] Factory CreateResponseHandler method returns a handler for TaskTypeMemory without error ✅
- [x] No compilation errors after changes ✅
- [x] Code passes `make lint` without violations ✅
- [x] Existing factory tests continue to pass ✅
- [x] Memory task execution no longer fails with "unsupported task type" errors ✅
