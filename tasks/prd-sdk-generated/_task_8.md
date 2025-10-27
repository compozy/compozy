## status: pending

<task_context>
<domain>sdk/workflow</domain>
<type>implementation</type>
<scope>code_generation</scope>
<complexity>medium</complexity>
<dependencies>sdk2/model</dependencies>
</task_context>

# Task 8.0: Migrate workflow Package to Functional Options

## Overview

Migrate `sdk/workflow` for orchestrating multiple tasks. Workflows define execution flow with tasks, transitions, and error handling.

**Estimated Time:** 2-3 hours

**Dependency:** Requires Task 1.0 (model) complete

<requirements>
- Generate options from engine/workflow/config.go
- Validate workflow ID and tasks collection
- Handle task references and dependencies
- Validate transition logic
- Deep copy with task collections
- Comprehensive tests
</requirements>

## Subtasks

- [ ] 8.1 Create sdk2/workflow/ directory structure
- [ ] 8.2 Create generate.go
- [ ] 8.3 Generate options (~10 fields)
- [ ] 8.4 Constructor with workflow validation
- [ ] 8.5 Task collection handling
- [ ] 8.6 Tests for workflow patterns
- [ ] 8.7 Document and verify

## Implementation Details

### Engine Fields (~10 fields)
- ID, Tasks (collection), InitialTask, Env, Timeout, ErrorHandling, Retry, Memory, Knowledge

### Key Validation
- At least one task required
- InitialTask must reference valid task
- Task IDs unique within workflow
- No circular task dependencies

### Collection Handling
```go
func New(ctx context.Context, id string, opts ...Option) (*workflow.Config, error) {
    cfg := &workflow.Config{
        ID: id,
        Tasks: make([]task.Config, 0),
    }
    // Apply options
    // Validate task collection
    // Validate initial task reference
}
```

### Relevant Files

**Reference (for understanding):**
- `sdk/workflow/builder.go` - Old builder pattern to understand requirements (~198 LOC)
- `sdk/workflow/builder_test.go` - Old tests to understand test cases
- `engine/workflow/config.go` - Source struct for generation

**To Create in sdk2/workflow/:**
- `generate.go` - Code generation directive
- `options_generated.go` - Generated functional options
- `constructors.go` - New() constructor with validation
- `constructors_test.go` - Comprehensive tests
- `README.md` - Documentation

**Note:** Do NOT delete or modify anything in `sdk/workflow/` - keep for reference during transition

## Tests

- [x] Workflow with single task
- [x] Workflow with multiple tasks
- [x] Task dependency validation
- [x] Initial task validation
- [x] Circular dependency detection
- [x] Empty workflow fails
- [x] Invalid initial task fails
- [x] Duplicate task IDs fail

## Success Criteria

- [x] sdk2/workflow/ directory structure created
- [x] Task collection validated
- [x] Initial task reference checked
- [x] Circular dependencies caught
- [x] Tests pass: `gotestsum -- ./sdk2/workflow`
- [x] Linter clean: `golangci-lint run ./sdk2/workflow/...`
- [x] Reduction: ~203 LOC → ~89 LOC (56% reduction, 17 fields generated)
- [x] Old sdk/workflow/ remains untouched
