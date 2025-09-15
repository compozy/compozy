# `engine/task2` – _Advanced Task Normalization and Response Processing_

> **A sophisticated task configuration normalization and response processing system that provides unified interfaces for all task types, context-aware template processing, and comprehensive workflow orchestration support.**

---

## 📑 Table of Contents

- [`engine/task2` – _Advanced Task Normalization and Response Processing_](#enginetask2--advanced-task-normalization-and-response-processing)
  - [📑 Table of Contents](#-table-of-contents)
  - [🎯 Overview](#-overview)
  - [💡 Motivation](#-motivation)
  - [⚡ Design Highlights](#-design-highlights)
  - [🚀 Getting Started](#-getting-started)
  - [📖 Usage](#-usage)
    - [Library](#library)
    - [Configuration Orchestration](#configuration-orchestration)
    - [Task Normalization](#task-normalization)
    - [Response Processing](#response-processing)
    - [Factory Pattern](#factory-pattern)
  - [🎨 Examples](#-examples)
    - [Complete Workflow Normalization](#complete-workflow-normalization)
    - [Signal-Based Task Processing](#signal-based-task-processing)
    - [Custom Response Processing](#custom-response-processing)
  - [📚 API Reference](#-api-reference)
    - [Core Interfaces](#core-interfaces)
      - [`Factory`](#factory)
      - [`ConfigOrchestrator`](#configorchestrator)
    - [Factory Implementation](#factory-implementation)
      - [`DefaultNormalizerFactory`](#defaultnormalizerfactory)
      - [`FactoryConfig`](#factoryconfig)
    - [Task Type Support](#task-type-support)
    - [Normalization Contracts](#normalization-contracts)
      - [`TaskNormalizer`](#tasknormalizer)
      - [`TaskResponseHandler`](#taskresponsehandler)
    - [Utility Functions](#utility-functions)
      - [`BuildTaskConfigsMap`](#buildtaskconfigsmap)
  - [🧪 Testing](#-testing)
    - [Test Categories](#test-categories)
    - [Test Structure](#test-structure)
  - [📦 Contributing](#-contributing)
  - [📄 License](#-license)

---

## 🎯 Overview

The `engine/task2` package provides advanced task configuration normalization and response processing capabilities for the Compozy workflow engine. It implements a factory-based architecture that supports all task types (basic, parallel, collection, composite, router, wait, signal, memory, and aggregate) with unified interfaces for normalization and response handling.

This package serves as the next-generation task processing layer, providing sophisticated template processing, context-aware normalization, and comprehensive workflow orchestration capabilities that enable complex AI-powered workflows.

---

## 💡 Motivation

- **Unified Interface**: Provide consistent interfaces for all task types and operations
- **Advanced Normalization**: Context-aware template processing with comprehensive variable support
- **Response Processing**: Sophisticated response handling with output transformation
- **Extensibility**: Factory pattern enables easy addition of new task types and features
- **Context Awareness**: Deep integration with workflow state and task hierarchies

---

## ⚡ Design Highlights

- **Factory Architecture**: Centralized factory for creating normalizers and response handlers
- **Template Processing**: Advanced template engine integration with context-aware variables
- **Comprehensive Normalization**: Support for all task types with type-specific processing
- **Response Handling**: Unified response processing with output transformation capabilities
- **Context Building**: Sophisticated context management for template processing
- **Signal Integration**: Advanced signal processing for wait tasks and workflow coordination

---

## 🚀 Getting Started

The task2 package is designed to work within the Compozy workflow engine ecosystem:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/compozy/compozy/engine/task"
    "github.com/compozy/compozy/engine/task2"
    "github.com/compozy/compozy/engine/task2/core"
    "github.com/compozy/compozy/engine/workflow"
    "github.com/compozy/compozy/pkg/tplengine"
)

func main() {
    // Create template engine
    templateEngine := tplengine.New()

    // Create factory
    factory, err := task2.NewFactory(&task2.FactoryConfig{
        TemplateEngine: templateEngine,
        EnvMerger:      core.NewEnvMerger(),
    })
    if err != nil {
        log.Fatal(err)
    }

    // Create orchestrator
    orchestrator, err := task2.NewConfigOrchestrator(factory)
    if err != nil {
        log.Fatal(err)
    }

    // Create workflow and task configurations
    workflowConfig := &workflow.Config{
        ID: "demo-workflow",
        // ... workflow configuration
    }

    workflowState := &workflow.State{
        WorkflowID: "demo-workflow",
        // ... workflow state
    }

    taskConfig := &task.Config{
        BaseConfig: task.BaseConfig{
            ID:   "demo-task",
            Type: task.TaskTypeBasic,
            With: &core.Input{
                "message": "{{ .workflow.input.greeting }}",
            },
        },
        // ... task configuration
    }

    // Normalize task configuration
    err = orchestrator.NormalizeTask(workflowState, workflowConfig, taskConfig)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Task configuration normalized successfully")
}
```

---

## 📖 Usage

### Library

The task2 package provides several core components:

```go
// Factory interface for creating components
type Factory interface {
    CreateNormalizer(taskType task.Type) (contracts.TaskNormalizer, error)
    CreateResponseHandler(ctx context.Context, taskType task.Type) (shared.TaskResponseHandler, error)
    // ... other factory methods
}

// Configuration orchestrator
type ConfigOrchestrator struct { /* ... */ }

// Core interfaces
type TaskNormalizer interface { /* ... */ }
type TaskResponseHandler interface { /* ... */ }
```

### Configuration Orchestration

The `ConfigOrchestrator` provides high-level orchestration of normalization processes:

```go
// Create orchestrator
orchestrator, err := task2.NewConfigOrchestrator(factory)
if err != nil {
    log.Fatal(err)
}

// Normalize a task
err = orchestrator.NormalizeTask(workflowState, workflowConfig, taskConfig)
if err != nil {
    log.Fatal(err)
}

// Normalize agent component
err = orchestrator.NormalizeAgentComponent(
    workflowState,
    workflowConfig,
    taskConfig,
    agentConfig,
    allTaskConfigs,
)
if err != nil {
    log.Fatal(err)
}

// Normalize tool component
err = orchestrator.NormalizeToolComponent(
    workflowState,
    workflowConfig,
    taskConfig,
    toolConfig,
    allTaskConfigs,
)
if err != nil {
    log.Fatal(err)
}
```

### Task Normalization

Different task types have specialized normalization logic:

```go
// Basic task normalization
basicNormalizer, err := factory.CreateNormalizer(task.TaskTypeBasic)
if err != nil {
    log.Fatal(err)
}

// Parallel task normalization
parallelNormalizer, err := factory.CreateNormalizer(task.TaskTypeParallel)
if err != nil {
    log.Fatal(err)
}

// Collection task normalization
collectionNormalizer, err := factory.CreateNormalizer(task.TaskTypeCollection)
if err != nil {
    log.Fatal(err)
}

// Wait task normalization with signal support
waitNormalizer, err := factory.CreateNormalizer(task.TaskTypeWait)
if err != nil {
    log.Fatal(err)
}

// Normalize with signal context
err = orchestrator.NormalizeTaskWithSignal(
    taskConfig,
    workflowState,
    workflowConfig,
    signalData,
)
if err != nil {
    log.Fatal(err)
}
```

### Response Processing

Response handlers provide unified response processing:

```go
// Create response handler
responseHandler, err := factory.CreateResponseHandler(ctx, task.TaskTypeBasic)
if err != nil {
    log.Fatal(err)
}

// Process task response
response, err := responseHandler.HandleResponse(
    ctx,
    taskState,
    taskConfig,
    workflowConfig,
)
if err != nil {
    log.Fatal(err)
}

// Transform output
transformedOutput, err := orchestrator.NormalizeTaskOutput(
    taskOutput,
    outputsConfig,
    workflowState,
    workflowConfig,
    taskConfig,
)
if err != nil {
    log.Fatal(err)
}
```

### Factory Pattern

The factory provides centralized creation of all components:

```go
// Create factory with full configuration
factory, err := task2.NewFactory(&task2.FactoryConfig{
    TemplateEngine: templateEngine,
    EnvMerger:      envMerger,
    WorkflowRepo:   workflowRepo,
    TaskRepo:       taskRepo,
})
if err != nil {
    log.Fatal(err)
}

// Create component normalizers
agentNormalizer := factory.CreateAgentNormalizer()
toolNormalizer := factory.CreateToolNormalizer()
outputTransformer := factory.CreateOutputTransformer()

// Create transition normalizers
successNormalizer := factory.CreateSuccessTransitionNormalizer()
errorNormalizer := factory.CreateErrorTransitionNormalizer()

// Create domain services
collectionExpander := factory.CreateCollectionExpander()
cwd, _ := core.CWDFromPath("/path/to/project")
configRepo, err := factory.CreateTaskConfigRepository(configStore, cwd)
if err != nil {
    log.Fatal(err)
}
```

---

## 🎨 Examples

### Complete Workflow Normalization

```go
// Create comprehensive workflow with multiple task types
func normalizeComplexWorkflow() error {
    // Create factory and orchestrator
    factory, err := task2.NewFactory(&task2.FactoryConfig{
        TemplateEngine: templateEngine,
        EnvMerger:      core.NewEnvMerger(),
        WorkflowRepo:   workflowRepo,
        TaskRepo:       taskRepo,
    })
    if err != nil {
        return err
    }

    orchestrator, err := task2.NewConfigOrchestrator(factory)
    if err != nil {
        return err
    }

    // Define workflow state
    workflowState := &workflow.State{
        WorkflowID: "data-processing",
        Input: &core.Input{
            "data_source": "api",
            "batch_size": 100,
            "priority": "high",
        },
        Output: &core.Output{},
        Tasks: make(map[string]*task.State),
    }

    // Define workflow config
    workflowConfig := &workflow.Config{
        ID: "data-processing",
        Tasks: []task.Config{
            // Basic task
            {
                BaseConfig: task.BaseConfig{
                    ID:   "validate-input",
                    Type: task.TaskTypeBasic,
                    With: &core.Input{
                        "source": "{{ .workflow.input.data_source }}",
                        "priority": "{{ .workflow.input.priority }}",
                    },
                },
                Agent: &agent.Config{
                    ID:           "validator",
                    Model:        "claude-3-5-haiku-latest",
                    Instructions: "Validate the input data",
                },
            },
            // Collection task
            {
                BaseConfig: task.BaseConfig{
                    ID:   "process-batch",
                    Type: task.TaskTypeCollection,
                    With: &core.Input{
                        "items": "{{ .workflow.input.batch_items }}",
                        "batch_size": "{{ .workflow.input.batch_size }}",
                    },
                },
                Collection: &task.CollectionConfig{
                    Items: "{{ .task.with.items }}",
                    ItemTemplate: &task.Config{
                        BaseConfig: task.BaseConfig{
                            ID: "process-item",
                            Agent: &agent.Config{
                                ID:           "processor",
                                Model:        "claude-3-5-haiku-latest",
                                Instructions: "Process individual item",
                            },
                            With: &core.Input{
                                "item": "{{ .collection.item }}",
                                "index": "{{ .collection.index }}",
                            },
                        },
                    },
                },
            },
            // Wait task
            {
                BaseConfig: task.BaseConfig{
                    ID:   "wait-approval",
                    Type: task.TaskTypeWait,
                },
                WaitConfig: &task.WaitConfig{
                    Conditions: []task.WaitCondition{
                        {
                            Expression: "signal.payload.approved == true",
                            Action:     "continue",
                        },
                    },
                    Timeout: "30m",
                },
            },
        },
    }

    // Normalize all tasks
    for i := range workflowConfig.Tasks {
        taskConfig := &workflowConfig.Tasks[i]

        err := orchestrator.NormalizeTask(workflowState, workflowConfig, taskConfig)
        if err != nil {
            return fmt.Errorf("failed to normalize task %s: %w", taskConfig.ID, err)
        }

        // Normalize agent if present
        if taskConfig.Agent != nil {
            allTaskConfigs := task2.BuildTaskConfigsMap(workflowConfig.Tasks)
            err := orchestrator.NormalizeAgentComponent(
                workflowState,
                workflowConfig,
                taskConfig,
                taskConfig.Agent,
                allTaskConfigs,
            )
            if err != nil {
                return fmt.Errorf("failed to normalize agent for task %s: %w", taskConfig.ID, err)
            }
        }

        // Normalize tool if present
        if taskConfig.Tool != nil {
            allTaskConfigs := task2.BuildTaskConfigsMap(workflowConfig.Tasks)
            err := orchestrator.NormalizeToolComponent(
                workflowState,
                workflowConfig,
                taskConfig,
                taskConfig.Tool,
                allTaskConfigs,
            )
            if err != nil {
                return fmt.Errorf("failed to normalize tool for task %s: %w", taskConfig.ID, err)
            }
        }
    }

    return nil
}
```

### Signal-Based Task Processing

```go
// Process wait task with signal
func processWaitTaskWithSignal() error {
    // Create signal data
    signalData := map[string]any{
        "payload": map[string]any{
            "user_id": "123",
            "approved": true,
            "comment": "Looks good to proceed",
        },
        "metadata": map[string]any{
            "signal_id": "sig-456",
            "timestamp": time.Now().Unix(),
        },
    }

    // Create wait task configuration
    waitTaskConfig := &task.Config{
        BaseConfig: task.BaseConfig{
            ID:   "approval-wait",
            Type: task.TaskTypeWait,
            With: &core.Input{
                "timeout": "1h",
                "required_approver": "manager",
            },
        },
        WaitConfig: &task.WaitConfig{
            Conditions: []task.WaitCondition{
                {
                    Expression: "signal.payload.approved == true && signal.payload.user_id == '123'",
                    Action:     "continue",
                    Processor: &task.Config{
                        BaseConfig: task.BaseConfig{
                            ID: "approval-processor",
                            Agent: &agent.Config{
                                ID:           "approver",
                                Model:        "claude-3-5-haiku-latest",
                                Instructions: "Process approval signal",
                            },
                            With: &core.Input{
                                "signal": "{{ .signal }}",
                                "context": "{{ .task.with }}",
                            },
                        },
                    },
                },
            },
        },
    }

    // Normalize with signal context
    err := orchestrator.NormalizeTaskWithSignal(
        waitTaskConfig,
        workflowState,
        workflowConfig,
        signalData,
    )
    if err != nil {
        return fmt.Errorf("failed to normalize wait task with signal: %w", err)
    }

    return nil
}
```

### Custom Response Processing

```go
// Create custom response handler
func createCustomResponseHandler() error {
    // Create response handler for basic task
    responseHandler, err := factory.CreateResponseHandler(ctx, task.TaskTypeBasic)
    if err != nil {
        return err
    }

    // Create mock task state
    taskState := &task.State{
        TaskID:         "custom-task",
        TaskExecID:     core.NewID(),
        WorkflowID:     "custom-workflow",
        WorkflowExecID: core.NewID(),
        Status:         core.StatusCompleted,
        Component:      core.ComponentAgent,
        ExecutionType:  task.ExecutionBasic,
        Output: &core.Output{
            "result": "processed data",
            "confidence": 0.95,
            "metadata": map[string]any{
                "tokens_used": 150,
                "model": "claude-3-5-haiku-latest",
            },
        },
    }

    // Create task config with output transformation
    taskConfig := &task.Config{
        BaseConfig: task.BaseConfig{
            ID:   "custom-task",
            Type: task.TaskTypeBasic,
            Outputs: &core.Input{
                "processed_result": "{{ .task.output.result }}",
                "high_confidence": "{{ .task.output.confidence > 0.9 }}",
                "summary": "{{ .task.output.result | truncate(100) }}",
            },
        },
    }

    // Process response
    response, err := responseHandler.HandleResponse(
        ctx,
        taskState,
        taskConfig,
        workflowConfig,
    )
    if err != nil {
        return fmt.Errorf("failed to handle response: %w", err)
    }

    fmt.Printf("Processed response: %+v\n", response)
    return nil
}
```

---

## 📚 API Reference

### Core Interfaces

#### `Factory`

```go
type Factory interface {
    CreateNormalizer(taskType task.Type) (contracts.TaskNormalizer, error)
    CreateResponseHandler(ctx context.Context, taskType task.Type) (shared.TaskResponseHandler, error)
    CreateAgentNormalizer() *core.AgentNormalizer
    CreateToolNormalizer() *core.ToolNormalizer
    CreateSuccessTransitionNormalizer() *core.SuccessTransitionNormalizer
    CreateErrorTransitionNormalizer() *core.ErrorTransitionNormalizer
    CreateOutputTransformer() *core.OutputTransformer
    CreateCollectionExpander() shared.CollectionExpander
    CreateTaskConfigRepository(configStore core.ConfigStore, cwd *core.PathCWD) (shared.TaskConfigRepository, error)
}
```

Central factory for creating all task2 components.

#### `ConfigOrchestrator`

```go
type ConfigOrchestrator struct { /* ... */ }

func NewConfigOrchestrator(factory Factory) (*ConfigOrchestrator, error)
```

High-level orchestrator for configuration normalization.

**Methods:**

- `NormalizeTask(workflowState, workflowConfig, taskConfig) error`
- `NormalizeAgentComponent(...) error`
- `NormalizeToolComponent(...) error`
- `NormalizeSuccessTransition(...) error`
- `NormalizeErrorTransition(...) error`
- `NormalizeTaskOutput(...) (*core.Output, error)`
- `NormalizeTaskWithSignal(...) error`
- `ClearCache()`

### Factory Implementation

#### `DefaultNormalizerFactory`

```go
type DefaultNormalizerFactory struct { /* ... */ }

func NewFactory(config *FactoryConfig) (Factory, error)
```

Default implementation of the Factory interface.

#### `FactoryConfig`

```go
type FactoryConfig struct {
    TemplateEngine *tplengine.TemplateEngine
    EnvMerger      *core.EnvMerger
    WorkflowRepo   workflow.Repository
    TaskRepo       task.Repository
}
```

Configuration for factory creation.

### Task Type Support

The package supports all Compozy task types:

```go
// Supported task types
task.TaskTypeBasic      // Basic agent/tool execution
task.TaskTypeParallel   // Parallel execution
task.TaskTypeCollection // Collection processing
task.TaskTypeComposite  // Composite tasks
task.TaskTypeRouter     // Conditional routing
task.TaskTypeWait       // Signal-based waiting
task.TaskTypeSignal     // Signal processing
task.TaskTypeMemory     // Memory operations
task.TaskTypeAggregate  // Aggregate operations
```

### Normalization Contracts

#### `TaskNormalizer`

```go
type TaskNormalizer interface {
    Normalize(config *task.Config, ctx *shared.NormalizationContext) error
}
```

Interface for task-specific normalization logic.

#### `TaskResponseHandler`

```go
type TaskResponseHandler interface {
    HandleResponse(
        ctx context.Context,
        state *task.State,
        config *task.Config,
        workflowConfig *workflow.Config,
    ) (map[string]any, error)
}
```

Interface for task response processing.

### Utility Functions

#### `BuildTaskConfigsMap`

```go
func BuildTaskConfigsMap(taskConfigSlice []task.Config) map[string]*task.Config
```

Converts task config slice to map for efficient lookups.

---

## 🧪 Testing

Run the test suite:

```bash
# Run all tests
go test ./engine/task2/...

# Run with verbose output
go test -v ./engine/task2/...

# Run specific test categories
go test -v ./engine/task2/basic
go test -v ./engine/task2/parallel
go test -v ./engine/task2/collection

# Run with coverage
go test -cover ./engine/task2/...

# Run integration tests
go test -v ./test/integration/task2
```

### Test Categories

The test suite covers:

- **Factory Creation**: Factory instantiation and component creation
- **Normalization**: All task type normalization scenarios
- **Response Processing**: Response handler functionality
- **Template Processing**: Context-aware template rendering
- **Signal Processing**: Wait task signal handling
- **Error Handling**: Comprehensive error scenarios
- **Integration**: End-to-end workflow processing

### Test Structure

Each task type has comprehensive test coverage:

```
task2/
├── basic/
│   ├── normalizer_test.go
│   ├── response_handler_test.go
│   └── context_builder_test.go
├── parallel/
│   ├── normalizer_test.go
│   ├── response_handler_test.go
│   └── context_builder_test.go
├── collection/
│   ├── normalizer_test.go
│   ├── response_handler_test.go
│   ├── expander_test.go
│   └── filter_evaluator_test.go
└── shared/
    ├── base_normalizer_test.go
    ├── context_test.go
    └── validation_test.go
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

## 📄 License

BSL-1.1 License - see [LICENSE](../../LICENSE)
