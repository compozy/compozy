## status: pending

<task_context>
<domain>sdk/cmd</domain>
<type>implementation</type>
<scope>examples</scope>
<complexity>low</complexity>
<dependencies>sdk2/model,sdk2/workflow</dependencies>
</task_context>

# Task 12.0: Create Consolidated Examples with Functional Options

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

## Example Functions to Implement

**Note:** All functions in single `sdk2/examples/main.go` file

### 1. RunSimpleWorkflow(ctx context.Context) error
**Demonstrates:**
- Basic agent creation with functional options
- Simple workflow configuration
- Model configuration

**Complexity:** Simple (1 agent, 1 workflow)

### 2. RunParallelTasks(ctx context.Context) error
**Demonstrates:**
- Parallel task configuration
- Multiple task execution

**Complexity:** Simple (parallel tasks)

### 3. RunKnowledgeRag(ctx context.Context) error
**Demonstrates:**
- Agent with knowledge base
- Knowledge binding configuration
- RAG pattern

**Complexity:** Medium (knowledge integration)

### 4. RunMemoryConversation(ctx context.Context) error
**Demonstrates:**
- Agent with memory
- Memory configuration
- Conversation persistence

**Complexity:** Medium (memory integration)

### 5. RunRuntimeNativeTools(ctx context.Context) error
**Demonstrates:**
- Runtime configuration
- Native tool definitions
- Tool execution

**Complexity:** Medium (runtime + tools)

### 6. RunScheduledWorkflow(ctx context.Context) error
**Demonstrates:**
- Schedule configuration
- Cron-based execution

**Complexity:** Simple (schedule integration)

### 7. RunSignalCommunication(ctx context.Context) error
**Demonstrates:**
- Signal tasks
- Workflow communication

**Complexity:** Medium (signal tasks)

### 8. RunCompleteProject(ctx context.Context) error
**Demonstrates:**
- Full project setup
- Multiple agents and workflows
- Complex configuration

**Complexity:** High (complete project)

### 9. RunDebugging(ctx context.Context) error
**Demonstrates:**
- Debugging configuration
- Error handling patterns

**Complexity:** Simple (debugging features)

## Testing Each Example

```bash
# Test each example via flag
go run sdk2/examples --example simple-workflow
go run sdk2/examples --example parallel-tasks
go run sdk2/examples --example knowledge-rag
go run sdk2/examples --example memory-conversation
go run sdk2/examples --example runtime-native-tools
go run sdk2/examples --example scheduled-workflow
go run sdk2/examples --example signal-communication
go run sdk2/examples --example complete-project
go run sdk2/examples --example debugging

# Show available examples
go run sdk2/examples --help

# Expected: Each example runs successfully without errors
# Verify: Output matches expected behavior
```

## Files

**Create:**
- `sdk2/examples/main.go` - Single file with all examples and CLI
- `sdk2/examples/README.md` - Documentation on running examples

**Structure:**
```
sdk2/examples/
├── main.go          # Main entry point with flag parsing
└── README.md        # Usage documentation
```

**Note:** Old examples in `sdk/cmd/` can remain for reference but will be deprecated in favor of the new consolidated approach

## Tests

- [ ] Main program compiles without errors
- [ ] Each example function runs via --example flag
- [ ] Help text displays all available examples
- [ ] Invalid example name shows helpful error
- [ ] Output behavior matches expected functionality
- [ ] No builder pattern references remaining
- [ ] All imports use sdk2 (sdk/v2/*)
- [ ] Engine types imported where needed
- [ ] Comments explain new patterns
- [ ] README.md provides clear usage instructions

## Success Criteria

- [ ] sdk2/examples/main.go created with all 9 example functions
- [ ] Program compiles: `go build sdk2/examples/main.go`
- [ ] All examples run: Each --example flag tested
- [ ] Help text shows all available examples
- [ ] No old builder pattern references
- [ ] Clean imports (no unused)
- [ ] Updated comments reflect new API
- [ ] Examples serve as learning resources
- [ ] README.md documents usage clearly
- [ ] Single command execution: `go run sdk2/examples --example <name>`
- [ ] Migration complete: Examples demonstrate sdk2 functional options

## Notes

**Critical Understanding:**
- We ARE creating a NEW consolidated examples program in `sdk2/examples/`
- Single main.go file with flag-based example selection
- Each example is an exported Run*() function
- Old examples in `sdk/cmd/` remain as reference but are deprecated
- Import pattern: All imports use `github.com/compozy/compozy/sdk/v2/*`

**Benefits of Single File Approach:**
- Easier for users to browse and understand all examples
- Single command to run any example: `go run sdk2/examples --example <name>`
- Easier maintenance (one file vs 9 directories)
- Clear API demonstration in one place
- Better discoverability via --help flag

**Execution:** Can be done in parallel with other tasks once Phase 1 complete
**User Impact:** High - examples are primary API documentation
**Testing:** Manual execution required for each example via flag
**Documentation:** README.md explains usage patterns and available examples
