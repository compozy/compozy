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

Create a single consolidated examples program in `sdk2/examples/` with a flag-based interface to run different examples. Each example will be an exported `Run<ExampleName>()` function, making it easier for users to browse code and run examples with a simple CLI.

**Estimated Time:** 2-3 hours

**Dependencies:** Requires Tasks 1.0-5.0 (Phase 1) complete minimum, ideally all migrations done

<critical>
- **USER-FACING:** Examples are primary learning resource
- **SINGLE ENTRY POINT:** One main.go with flag-based example selection
- **BETTER UX:** Users run `go run sdk2/examples --example <name>` instead of navigating multiple directories
- **CLEANER STRUCTURE:** All examples in one file, easier to maintain and browse
- **API DEMONSTRATION:** Must show best practices with new functional options
</critical>

<requirements>
- Create single sdk2/examples/ directory with main.go
- Each example as exported Run<ExampleName>(ctx context.Context) error function
- CLI flag --example <name> to select which example to run
- Follow new API: ctx first, no .Build(), WithX() options
- Use sdk2 imports throughout
- Verify each example still runs correctly via flag
- Maintain example functionality (no behavior changes)
- Clear help text showing available examples
</requirements>

## Subtasks

- [ ] 12.1 Create sdk2/examples/ directory and main.go with flag parsing
- [ ] 12.2 Implement RunSimpleWorkflow() function
- [ ] 12.3 Implement RunParallelTasks() function
- [ ] 12.4 Implement RunKnowledgeRag() function
- [ ] 12.5 Implement RunMemoryConversation() function
- [ ] 12.6 Implement RunRuntimeNativeTools() function
- [ ] 12.7 Implement RunScheduledWorkflow() function
- [ ] 12.8 Implement RunSignalCommunication() function
- [ ] 12.9 Implement RunCompleteProject() function
- [ ] 12.10 Implement RunDebugging() function
- [ ] 12.11 Add help text and example listing
- [ ] 12.12 Test all examples via --example flag

## Structure Pattern

### New Single File Structure
```
sdk2/
└── examples/
    └── main.go  (single file with all examples)
```

### Main.go Structure
```go
package main

import (
    "context"
    "flag"
    "fmt"
    "github.com/compozy/compozy/sdk/v2/agent"
    "github.com/compozy/compozy/sdk/v2/workflow"
    engineagent "github.com/compozy/compozy/engine/agent"
    "github.com/compozy/compozy/engine/core"
)

func main() {
    exampleName := flag.String("example", "", "Example to run: simple-workflow, parallel-tasks, knowledge-rag, ...")
    flag.Parse()

    ctx := context.Background()
    
    switch *exampleName {
    case "simple-workflow":
        err := RunSimpleWorkflow(ctx)
    case "parallel-tasks":
        err := RunParallelTasks(ctx)
    // ... other examples
    default:
        fmt.Println("Available examples: simple-workflow, parallel-tasks, ...")
    }
}

// RunSimpleWorkflow demonstrates basic workflow creation
func RunSimpleWorkflow(ctx context.Context) error {
    agentCfg, err := agent.New(ctx, "assistant",
        agent.WithInstructions("You are helpful"),
        agent.WithModel(engineagent.Model{
            Config: core.ProviderConfig{
                Provider: core.ProviderOpenAI,
                Model:    "gpt-4",
            },
        }),
    )
    // ... rest of example
}
```

### Usage
```bash
# Run specific example
go run sdk2/examples --example simple-workflow

# Show help
go run sdk2/examples --help
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
