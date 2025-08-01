# `engine/task` – _Task Execution and State Management Engine_

> **A comprehensive task execution system that provides state management, workflow orchestration, and hierarchical task execution capabilities for the Compozy workflow engine.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
  - [Library](#library)
  - [Task Configuration](#task-configuration)
  - [State Management](#state-management)
  - [Execution Types](#execution-types)
  - [Wait Tasks and Signals](#wait-tasks-and-signals)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `engine/task` package is the core task execution engine for Compozy workflows. It provides comprehensive task state management, hierarchical execution patterns, and sophisticated orchestration capabilities including parallel execution, collection processing, and signal-based coordination.

This package handles the complete lifecycle of task execution from configuration normalization through execution to completion, supporting both simple single-step tasks and complex multi-step workflows with parent-child relationships.

---

## 💡 Motivation

- **Workflow Orchestration**: Enable complex workflow patterns with hierarchical task relationships
- **State Management**: Provide robust state tracking and persistence for long-running workflows
- **Execution Patterns**: Support multiple execution strategies (basic, parallel, collection, composite)
- **Signal Coordination**: Enable task coordination through signal-based communication
- **Error Handling**: Comprehensive error handling with retry logic and fallback strategies

---

## ⚡ Design Highlights

- **Hierarchical Execution**: Support for parent-child task relationships and complex workflow patterns
- **Multiple Execution Types**: Basic, parallel, collection, composite, router, and wait execution strategies
- **State Persistence**: Comprehensive state management with database persistence
- **Signal Processing**: Advanced signal-based task coordination and workflow control
- **CEL Integration**: Common Expression Language support for dynamic conditions
- **Resource Management**: Efficient handling of agents, tools, and workflow resources

---

## 🚀 Getting Started

The task package is designed to work within the Compozy workflow engine ecosystem:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/engine/task"
)

func main() {
    // Create a basic task configuration
    taskConfig := &task.Config{
        BaseConfig: task.BaseConfig{
            ID:   "hello-world",
            Type: task.TaskTypeBasic,
        },
        Agent: &agent.Config{
            ID:           "greeting-agent",
            Model:        "claude-3-haiku-20240307",
            Instructions: "Generate a friendly greeting",
        },
        With: &core.Input{
            "name": "World",
            "style": "friendly",
        },
    }

    // Create task state
    state := task.CreateState(
        &task.CreateStateInput{
            TaskID:         "hello-world",
            TaskExecID:     core.NewID(),
            WorkflowID:     "demo-workflow",
            WorkflowExecID: core.NewID(),
        },
        &task.PartialState{
            Component:     core.ComponentAgent,
            ExecutionType: task.ExecutionBasic,
            Input:         taskConfig.With,
        },
    )

    fmt.Printf("Created task state: %+v\n", state)
}
```

---

## 📖 Usage

### Library

The task package provides several core components:

```go
// Core task types
type Config struct { /* comprehensive task configuration */ }
type State struct { /* task execution state */ }
type ExecutionType string // basic, parallel, collection, composite, etc.

// State creation
type CreateStateInput struct { /* input for state creation */ }
type PartialState struct { /* partial state for creation */ }

// Wait tasks and signals
type SignalEnvelope struct { /* signal data and metadata */ }
type WaitTaskResult struct { /* wait task execution result */ }

// Validation and processing
type ParamsValidator struct { /* parameter validation */ }
type ConditionEvaluator interface { /* CEL expression evaluation */ }
```

### Task Configuration

#### Basic Task Configuration

```go
// Create a basic task with agent execution
basicTask := &task.Config{
    BaseConfig: task.BaseConfig{
        ID:   "process-data",
        Type: task.TaskTypeBasic,
        With: &core.Input{
            "data": "{{ .workflow.input.raw_data }}",
            "format": "json",
        },
        Env: &core.EnvMap{
            "LOG_LEVEL": "info",
        },
    },
    Agent: &agent.Config{
        ID:           "data-processor",
        Model:        "claude-3-haiku-20240307",
        Instructions: "Process the input data and extract key information",
    },
    Input: &schema.Schema{
        "type": "object",
        "properties": map[string]any{
            "data": map[string]any{"type": "string"},
            "format": map[string]any{
                "type": "string",
                "enum": []string{"json", "xml", "csv"},
                "default": "json",
            },
        },
        "required": []string{"data"},
    },
}
```

#### Parallel Task Configuration

```go
// Create a parallel task that executes multiple subtasks
parallelTask := &task.Config{
    BaseConfig: task.BaseConfig{
        ID:   "parallel-processing",
        Type: task.TaskTypeParallel,
    },
    ParallelTasks: []task.ParallelTaskItem{
        {
            BaseConfig: task.BaseConfig{
                ID: "process-emails",
                Agent: &agent.Config{
                    ID:           "email-processor",
                    Model:        "claude-3-haiku-20240307",
                    Instructions: "Process email data",
                },
                With: &core.Input{
                    "emails": "{{ .workflow.input.emails }}",
                },
            },
        },
        {
            BaseConfig: task.BaseConfig{
                ID: "process-documents",
                Agent: &agent.Config{
                    ID:           "doc-processor",
                    Model:        "claude-3-haiku-20240307",
                    Instructions: "Process document data",
                },
                With: &core.Input{
                    "docs": "{{ .workflow.input.documents }}",
                },
            },
        },
    },
}
```

### State Management

#### Creating and Managing Task States

```go
// Create state for a basic task
createInput := &task.CreateStateInput{
    TaskID:         "my-task",
    TaskExecID:     core.NewID(),
    WorkflowID:     "my-workflow",
    WorkflowExecID: core.NewID(),
}

partialState := &task.PartialState{
    Component:     core.ComponentAgent,
    ExecutionType: task.ExecutionBasic,
    Input:         &core.Input{"key": "value"},
}

state := task.CreateState(createInput, partialState)

// Update state status
state.UpdateStatus(core.StatusRunning)

// Create child task state
childState := task.CreateSubTaskState(
    "child-task",
    core.NewID(),
    "my-workflow",
    state.WorkflowExecID,
    &state.TaskExecID, // parent state ID
    task.ExecutionBasic,
    core.ComponentTool,
    &core.Input{"child_data": "value"},
)
```

#### Working with Hierarchical States

```go
// Check if task can have children
if state.CanHaveChildren() {
    fmt.Println("Task supports child tasks")
}

// Check if task is a child
if state.IsChildTask() {
    fmt.Printf("Parent ID: %s\n", state.GetParentID())
}

// Check execution type
if state.IsParallelExecution() {
    fmt.Println("Parallel execution enabled")
}

// Validate parent-child relationships
if err := state.ValidateParentChild(parentID); err != nil {
    log.Fatal(err)
}
```

### Execution Types

The task package supports multiple execution patterns:

```go
// Basic execution - single agent or tool
basic := task.ExecutionBasic

// Parallel execution - multiple tasks in parallel
parallel := task.ExecutionParallel

// Collection execution - iterate over collections
collection := task.ExecutionCollection

// Composite execution - complex nested structures
composite := task.ExecutionComposite

// Router execution - conditional routing
router := task.ExecutionRouter

// Wait execution - signal-based coordination
wait := task.ExecutionWait
```

### Wait Tasks and Signals

#### Signal Processing

```go
// Create signal envelope
signal := &task.SignalEnvelope{
    Payload: map[string]any{
        "event": "user_action",
        "data": map[string]any{
            "user_id": "123",
            "action": "click",
        },
    },
    Metadata: task.SignalMetadata{
        SignalID:      "signal-123",
        ReceivedAtUTC: time.Now().UTC(),
        WorkflowID:    "workflow-456",
        Source:        "user-interface",
    },
}

// Process signal
processor := // ... implement SignalProcessor interface
result, err := processor.Process(context.Background(), signal)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Processing result: %+v\n", result)
```

#### Wait Task Execution

```go
// Configure wait task
waitConfig := &task.Config{
    BaseConfig: task.BaseConfig{
        ID:   "wait-for-approval",
        Type: task.TaskTypeWait,
    },
    WaitConfig: &task.WaitConfig{
        Conditions: []task.WaitCondition{
            {
                Expression: "signal.payload.approved == true",
                Action:     "continue",
            },
            {
                Expression: "signal.payload.rejected == true",
                Action:     "fail",
            },
        },
        Timeout: "1h",
    },
}

// Execute wait task
executor := // ... implement WaitTaskExecutor interface
result, err := executor.Execute(context.Background(), waitConfig)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Wait task result: %+v\n", result)
```

---

## 🎨 Examples

### Complex Workflow with Error Handling

```go
// Create a task with comprehensive error handling
taskWithRetry := &task.Config{
    BaseConfig: task.BaseConfig{
        ID:   "resilient-task",
        Type: task.TaskTypeBasic,
        With: &core.Input{
            "data": "{{ .workflow.input.data }}",
        },
        OnError: &core.ErrorTransition{
            Next:    "error-handler",
            Retry:   3,
            Backoff: "exponential",
            With: &core.Input{
                "error": "{{ .task.error }}",
                "attempt": "{{ .task.attempt }}",
            },
        },
        OnSuccess: &core.SuccessTransition{
            Next: "next-task",
            With: &core.Input{
                "result": "{{ .task.output }}",
            },
        },
    },
    Agent: &agent.Config{
        ID:           "processor",
        Model:        "claude-3-haiku-20240307",
        Instructions: "Process data with error handling",
    },
    Input: &schema.Schema{
        "type": "object",
        "properties": map[string]any{
            "data": map[string]any{"type": "string"},
        },
        "required": []string{"data"},
    },
}
```

### Collection Processing Task

```go
// Create collection task for batch processing
collectionTask := &task.Config{
    BaseConfig: task.BaseConfig{
        ID:   "process-batch",
        Type: task.TaskTypeCollection,
        With: &core.Input{
            "items": "{{ .workflow.input.batch_items }}",
        },
    },
    Collection: &task.CollectionConfig{
        Items: "{{ .task.with.items }}",
        ItemTemplate: &task.Config{
            BaseConfig: task.BaseConfig{
                ID: "process-item",
                Agent: &agent.Config{
                    ID:           "item-processor",
                    Model:        "claude-3-haiku-20240307",
                    Instructions: "Process individual item",
                },
                With: &core.Input{
                    "item": "{{ .collection.item }}",
                    "index": "{{ .collection.index }}",
                },
            },
        },
        Filter: "{{ .collection.item.enabled == true }}",
        Limit:  100,
    },
}
```

### Router Task with Conditional Logic

```go
// Create router task for conditional branching
routerTask := &task.Config{
    BaseConfig: task.BaseConfig{
        ID:   "route-request",
        Type: task.TaskTypeRouter,
        With: &core.Input{
            "request_type": "{{ .workflow.input.type }}",
            "priority": "{{ .workflow.input.priority }}",
        },
    },
    Router: &task.RouterConfig{
        Routes: []task.Route{
            {
                Condition: "{{ .task.with.request_type == 'urgent' }}",
                Task: &task.Config{
                    BaseConfig: task.BaseConfig{
                        ID: "urgent-handler",
                        Agent: &agent.Config{
                            ID:           "urgent-processor",
                            Model:        "claude-3-haiku-20240307",
                            Instructions: "Handle urgent requests",
                        },
                    },
                },
            },
            {
                Condition: "{{ .task.with.request_type == 'normal' }}",
                Task: &task.Config{
                    BaseConfig: task.BaseConfig{
                        ID: "normal-handler",
                        Agent: &agent.Config{
                            ID:           "normal-processor",
                            Model:        "claude-3-haiku-20240307",
                            Instructions: "Handle normal requests",
                        },
                    },
                },
            },
        },
        Default: &task.Config{
            BaseConfig: task.BaseConfig{
                ID: "default-handler",
                Agent: &agent.Config{
                    ID:           "default-processor",
                    Model:        "claude-3-haiku-20240307",
                    Instructions: "Handle default case",
                },
            },
        },
    },
}
```

---

## 📚 API Reference

### Core Types

#### `Config`

```go
type Config struct {
    BaseConfig
    // Configuration for different execution types
    Agent         *agent.Config
    Tool          *tool.Config
    ParallelTasks []ParallelTaskItem
    Collection    *CollectionConfig
    Router        *RouterConfig
    WaitConfig    *WaitConfig
    // ... other fields
}
```

Primary configuration structure for all task types.

#### `State`

```go
type State struct {
    // Core identification
    TaskID         string
    TaskExecID     core.ID
    WorkflowID     string
    WorkflowExecID core.ID

    // Execution details
    Component     core.ComponentType
    Status        core.StatusType
    ExecutionType ExecutionType

    // Parent-child relationships
    ParentStateID *core.ID

    // Execution data
    Input  *core.Input
    Output *core.Output
    Error  *core.Error

    // Timestamps
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

Represents the runtime state of a task execution.

#### `ExecutionType`

```go
type ExecutionType string

const (
    ExecutionBasic      ExecutionType = "basic"
    ExecutionRouter     ExecutionType = "router"
    ExecutionParallel   ExecutionType = "parallel"
    ExecutionCollection ExecutionType = "collection"
    ExecutionComposite  ExecutionType = "composite"
    ExecutionWait       ExecutionType = "wait"
)
```

Defines the execution strategy for tasks.

### State Creation Functions

#### `CreateState`

```go
func CreateState(input *CreateStateInput, result *PartialState) *State
```

Creates a new task state with the provided configuration.

#### `CreateSubTaskState`

```go
func CreateSubTaskState(
    taskID string,
    taskExecID core.ID,
    workflowID string,
    workflowExecID core.ID,
    parentStateID *core.ID,
    execType ExecutionType,
    component core.ComponentType,
    input *core.Input,
) *State
```

Creates a child task state with parent relationship.

#### `CreateAgentSubTaskState`

```go
func CreateAgentSubTaskState(
    taskID string,
    taskExecID core.ID,
    workflowID string,
    workflowExecID core.ID,
    parentStateID *core.ID,
    agentID, actionID string,
    input *core.Input,
) *State
```

Creates a child task state specifically for agent execution.

### State Methods

#### Hierarchy Methods

```go
func (s *State) CanHaveChildren() bool
func (s *State) IsChildTask() bool
func (s *State) IsParallelExecution() bool
func (s *State) GetParentID() *core.ID
func (s *State) ValidateParentChild(parentID core.ID) error
```

#### Status Methods

```go
func (s *State) UpdateStatus(status core.StatusType)
func (s *State) AsMap() (map[core.ID]any, error)
```

### Signal Processing

#### `SignalEnvelope`

```go
type SignalEnvelope struct {
    Payload  map[string]any
    Metadata SignalMetadata
}
```

Contains signal data and system metadata.

#### `SignalProcessor`

```go
type SignalProcessor interface {
    Process(ctx context.Context, signal *SignalEnvelope) (*ProcessorOutput, error)
}
```

Interface for processing signals in wait tasks.

#### `WaitTaskExecutor`

```go
type WaitTaskExecutor interface {
    Execute(ctx context.Context, config *Config) (*WaitTaskResult, error)
}
```

Interface for executing wait tasks.

---

## 🧪 Testing

Run the test suite:

```bash
# Run all tests
go test ./engine/task

# Run with verbose output
go test -v ./engine/task

# Run specific test categories
go test -v ./engine/task -run TestState
go test -v ./engine/task -run TestConfig
go test -v ./engine/task -run TestSignal

# Run with coverage
go test -cover ./engine/task

# Run integration tests
go test -v ./test/integration/task
```

### Test Categories

The test suite covers:

- **State Management**: Creation, updates, and hierarchy validation
- **Configuration**: Task configuration parsing and validation
- **Execution Types**: All execution pattern implementations
- **Signal Processing**: Wait task signal handling
- **Error Handling**: Comprehensive error scenarios
- **Performance**: Stress testing for large workflows

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

## 📄 License

BSL-1.1 License - see [LICENSE](../../LICENSE)
