## status: pending

<task_context>
<domain>sdk/cmd</domain>
<type>implementation</type>
<scope>examples</scope>
<complexity>low</complexity>
<dependencies>sdk2/model,sdk2/workflow</dependencies>
</task_context>

# Task 12.0: Update Example Files to Use Functional Options

## Overview

Update 9 example files in `sdk/cmd/` to use the new functional options API from `sdk2/` instead of the old builder pattern from `sdk/`. This demonstrates the new API to users and validates the migration.

**Estimated Time:** 1-2 hours

**Dependencies:** Requires Tasks 1.0-5.0 (Phase 1) complete minimum, ideally all migrations done

<critical>
- **USER-FACING:** Examples are primary learning resource
- **API DEMONSTRATION:** Must show best practices
- **UPDATING EXISTING EXAMPLES:** Modify sdk/cmd/ files to import from sdk2/ instead of sdk/
- **IMPORT MIGRATION:** Change from `github.com/compozy/compozy/sdk/X` to `github.com/compozy/compozy/sdk/v2/X`
</critical>

<requirements>
- Update all 9 example main.go files in sdk/cmd/
- Follow new API: ctx first, no .Build(), WithX() options
- Update imports: sdk → sdk2, add engine imports
- Verify each example still runs correctly
- Maintain example functionality (no behavior changes)
- Update comments to reflect new patterns
</requirements>

## Subtasks

- [ ] 12.1 Update 01_simple_workflow/main.go
- [ ] 12.2 Update 02_parallel_tasks/main.go
- [ ] 12.3 Update 03_knowledge_rag/main.go
- [ ] 12.4 Update 04_memory_conversation/main.go
- [ ] 12.5 Update 06_runtime_native_tools/main.go
- [ ] 12.6 Update 07_scheduled_workflow/main.go
- [ ] 12.7 Update 08_signal_communication/main.go
- [ ] 12.8 Update 10_complete_project/main.go
- [ ] 12.9 Update 11_debugging/main.go
- [ ] 12.10 Test all examples execute successfully

## Migration Pattern

### Before (Builder Pattern from sdk/)
```go
import "github.com/compozy/compozy/sdk/agent"

agentCfg, err := agent.New("assistant").
    WithModel("openai", "gpt-4").
    WithInstructions("You are helpful").
    AddAction(action).
    AddTool("tool1").
    Build(ctx)
```

### After (Functional Options from sdk2/)
```go
import (
    "github.com/compozy/compozy/sdk/v2/agent"  // Changed: sdk → sdk/v2
    engineagent "github.com/compozy/compozy/engine/agent"
    "github.com/compozy/compozy/engine/core"
    enginetool "github.com/compozy/compozy/engine/tool"
)

agentCfg, err := agent.New(ctx, "assistant",
    agent.WithInstructions("You are helpful"),
    agent.WithModel(engineagent.Model{
        Config: core.ProviderConfig{
            Provider: core.ProviderOpenAI,
            Model:    "gpt-4",
        },
    }),
    agent.WithActions([]*engineagent.ActionConfig{action}),
    agent.WithTools([]enginetool.Config{{ID: "tool1"}}),
)
```

**Key Import Change:**
```
FROM: import "github.com/compozy/compozy/sdk/agent"
TO:   import "github.com/compozy/compozy/sdk/v2/agent"
```

### Key Changes Checklist
- [ ] ✅ Change imports from `sdk/X` to `sdk/v2/X`
- [ ] ✅ `ctx` is first parameter
- [ ] ✅ No `.Build(ctx)` call
- [ ] ✅ Use `WithX()` functions
- [ ] ✅ `WithModel()` takes Model struct
- [ ] ✅ Collections use `WithXs()` (plural) with slices
- [ ] ✅ Import engine packages for types

## Example Files to Update

**Note:** All files are in `sdk/cmd/` and will be MODIFIED (not created) to use sdk2 imports

### 1. 01_simple_workflow/main.go
**Changes needed:**
- Update imports: sdk → sdk/v2
- Agent builder → functional options
- Workflow builder → functional options

**Complexity:** Simple (1 agent, 1 workflow)

### 2. 02_parallel_tasks/main.go
**Changes needed:**
- Update imports: sdk → sdk/v2
- Agent builder → functional options
- Parallel task builder → functional options

**Complexity:** Simple (parallel tasks)

### 3. 03_knowledge_rag/main.go
**Changes needed:**
- Update imports: sdk → sdk/v2
- Agent with knowledge → functional options
- Knowledge base config → functional options

**Complexity:** Medium (knowledge integration)

### 4. 04_memory_conversation/main.go
**Changes needed:**
- Update imports: sdk → sdk/v2
- Agent with memory → functional options
- Memory config → functional options

**Complexity:** Medium (memory integration)

### 5. 06_runtime_native_tools/main.go
**Changes needed:**
- Update imports: sdk → sdk/v2
- Agent with runtime tools → functional options
- Runtime config → functional options
- Tool definitions → functional options

**Complexity:** Medium (runtime + tools)

### 6. 07_scheduled_workflow/main.go
**Changes needed:**
- Update imports: sdk → sdk/v2
- Workflow → functional options
- Schedule config → functional options

**Complexity:** Simple (schedule integration)

### 7. 08_signal_communication/main.go
**Changes needed:**
- Update imports: sdk → sdk/v2
- Workflow with signals → functional options
- Signal task config → functional options

**Complexity:** Medium (signal tasks)

### 8. 10_complete_project/main.go
**Changes needed:**
- Update imports: sdk → sdk/v2
- Project builder → functional options
- All nested configs → functional options
- Multiple agents, workflows → functional options

**Complexity:** High (complete project)

### 9. 11_debugging/main.go
**Changes needed:**
- Update imports: sdk → sdk/v2
- Debugging config → functional options

**Complexity:** Simple (debugging features)

## Testing Each Example

```bash
# Test pattern for each example
cd sdk/cmd/01_simple_workflow
go run main.go

# Expected: Example runs successfully without errors
# Verify: Output matches expected behavior
```

## Files

**Update:**
- `sdk/cmd/01_simple_workflow/main.go`
- `sdk/cmd/02_parallel_tasks/main.go`
- `sdk/cmd/03_knowledge_rag/main.go`
- `sdk/cmd/04_memory_conversation/main.go`
- `sdk/cmd/06_runtime_native_tools/main.go`
- `sdk/cmd/07_scheduled_workflow/main.go`
- `sdk/cmd/08_signal_communication/main.go`
- `sdk/cmd/10_complete_project/main.go`
- `sdk/cmd/11_debugging/main.go`

**Note:** Example 05_mcp_integration already updated ✅

## Tests

- [ ] Each example compiles without errors
- [ ] Each example runs successfully
- [ ] Output behavior unchanged from original
- [ ] No builder pattern references remaining
- [ ] All imports updated to sdk2
- [ ] Engine types imported where needed
- [ ] Comments explain new patterns

## Success Criteria

- [ ] All 9 examples updated
- [ ] All examples compile: `go build ./sdk/cmd/*/main.go`
- [ ] All examples run: Manual execution of each
- [ ] No old builder pattern references
- [ ] Clean imports (no unused)
- [ ] Updated comments reflect new API
- [ ] Examples serve as learning resources
- [ ] Migration complete: 100% SDK using functional options

## Notes

**Critical Understanding:**
- We ARE modifying files in `sdk/cmd/` (existing examples)
- We are NOT creating new examples from scratch
- Purpose: Update existing examples to demonstrate the new sdk2 API
- Import changes: `sdk/X` → `sdk/v2/X` throughout

**Execution:** Can be done in parallel with other tasks once Phase 1 complete
**User Impact:** High - examples are primary API documentation
**Testing:** Manual execution required for each example
**Documentation:** Consider adding migration guide comments in examples
