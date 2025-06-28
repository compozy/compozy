# Task Services Architecture Technical Specification

## Executive Summary

This specification details the implementation of a task-type-specific orchestration architecture for Compozy. Instead of shared services with type-specific methods and switch statements, each task type will have its own orchestrator implementation behind clean interfaces. This eliminates violations of SOLID principles and creates a truly extensible system where new task types can be added without modifying existing code.

## System Architecture

### Domain Placement

Components will be organized within `engine/` as follows:

- **engine/task2/interfaces/** - Core interfaces and contracts
- **engine/task2/factory/** - Orchestrator factory and registration
- **engine/task2/parallel/** - Parallel task orchestration
- **engine/task2/collection/** - Collection task orchestration
- **engine/task2/wait/** - Wait task orchestration
- **engine/task2/signal/** - Signal task orchestration
- **engine/task2/basic/** - Basic task orchestration
- **engine/task2/composite/** - Composite task orchestration
- **engine/task2/aggregate/** - Aggregate task orchestration
- **engine/task2/router/** - Router task orchestration
- **engine/task2/shared/** - Shared utilities and base implementations

### Component Overview

**Orchestrator Interface**: Core contract that all task types implement

- Manages complete task lifecycle
- Handles state creation and execution
- Processes responses and transitions

**Task-Specific Orchestrators**: Each task type's complete implementation

- Encapsulates all type-specific logic
- No dependencies on other task types
- Implements only needed interfaces

**Optional Interfaces**: Capabilities that only some tasks need

- ChildTaskManager for container tasks
- SignalHandler for signal-based tasks
- StatusAggregator for parent tasks

**Orchestrator Factory**: Creates appropriate orchestrator instances

- Maps task types to implementations
- Supports dynamic registration
- No knowledge of implementation details

## Implementation Design

### Core Interfaces

```go
// engine/task2/interfaces/orchestrator.go
package interfaces

import (
    "context"
    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/engine/task"
    "github.com/compozy/compozy/engine/workflow"
)

// TaskOrchestrator is the core interface all task types must implement
type TaskOrchestrator interface {
    // CreateState creates the initial task state
    CreateState(ctx context.Context, input CreateStateInput) (*task.State, error)

    // PrepareExecution prepares any resources needed before execution
    PrepareExecution(ctx context.Context, state *task.State) error

    // HandleResponse processes the task execution response
    HandleResponse(ctx context.Context, input HandleResponseInput) (*task.Response, error)

    // GetType returns the task type this orchestrator handles
    GetType() task.TaskType
}

type CreateStateInput struct {
    WorkflowState  *workflow.State
    WorkflowConfig *workflow.Config
    TaskConfig     *task.Config
    ParentStateID  *core.ID // Optional, for child tasks
}

type HandleResponseInput struct {
    State          *task.State
    ExecutionError error
    Output         *core.Output
}
```

```go
// engine/task2/interfaces/child_manager.go
package interfaces

// ChildTaskManager is implemented by tasks that create child tasks
type ChildTaskManager interface {
    // PrepareChildren prepares child task configurations
    PrepareChildren(ctx context.Context, parent *task.State, config *task.Config) error

    // CreateChildren creates the actual child task states
    CreateChildren(ctx context.Context, parentID core.ID) ([]*task.State, error)

    // GetChildrenMetadata returns metadata about prepared children
    GetChildrenMetadata(ctx context.Context, parentID core.ID) (ChildrenMetadata, error)
}

type ChildrenMetadata struct {
    Count        int
    Strategy     string
    MaxWorkers   int
    CustomFields map[string]interface{} // Task-specific metadata
}
```

```go
// engine/task2/interfaces/signal_handler.go
package interfaces

// SignalHandler is implemented by tasks that process signals
type SignalHandler interface {
    // ValidateSignal checks if a signal can be processed
    ValidateSignal(ctx context.Context, state *task.State, signal Signal) error

    // ProcessSignal handles an incoming signal
    ProcessSignal(ctx context.Context, state *task.State, signal Signal) (*task.State, error)
}

type Signal struct {
    Name          string
    Payload       map[string]interface{}
    CorrelationID string
    Timestamp     time.Time
}
```

```go
// engine/task2/interfaces/status_aggregator.go
package interfaces

// StatusAggregator is implemented by tasks that aggregate child status
type StatusAggregator interface {
    // CalculateStatus determines parent status from children
    CalculateStatus(ctx context.Context, parentID core.ID) (core.StatusType, error)

    // ShouldUpdateStatus checks if status update is needed
    ShouldUpdateStatus(ctx context.Context, parentID core.ID, childUpdate ChildStatusUpdate) bool
}

type ChildStatusUpdate struct {
    ChildID     core.ID
    OldStatus   core.StatusType
    NewStatus   core.StatusType
    Timestamp   time.Time
}
```

### Data Models

```go
// engine/task2/shared/models.go
package shared

// OrchestratorContext carries dependencies for orchestrators
type OrchestratorContext struct {
    TaskRepo       task.Repository
    WorkflowRepo   workflow.Repository
    Storage        Storage
    TemplateEngine *tplengine.TemplateEngine
}

// Storage interface for task metadata
type Storage interface {
    Store(ctx context.Context, key string, value interface{}) error
    Load(ctx context.Context, key string, dest interface{}) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
}
```

### Task-Specific Implementations

#### Parallel Task Orchestrator

```go
// engine/task2/parallel/orchestrator.go
package parallel

type Orchestrator struct {
    *shared.BaseOrchestrator
    childPreparer *ChildPreparer
    statusCalc    *StatusCalculator
}

func NewOrchestrator(ctx *shared.OrchestratorContext) *Orchestrator {
    return &Orchestrator{
        BaseOrchestrator: shared.NewBaseOrchestrator(ctx, task.TaskTypeParallel),
        childPreparer:    NewChildPreparer(ctx.Storage),
        statusCalc:       NewStatusCalculator(ctx.TaskRepo),
    }
}

func (o *Orchestrator) CreateState(ctx context.Context, input interfaces.CreateStateInput) (*task.State, error) {
    // Create parent state
    state, err := o.BaseOrchestrator.CreateState(ctx, input)
    if err != nil {
        return nil, err
    }

    // Prepare children for parallel execution
    if err := o.childPreparer.PrepareChildren(ctx, state, input.TaskConfig); err != nil {
        return nil, fmt.Errorf("failed to prepare parallel children: %w", err)
    }

    return state, nil
}

func (o *Orchestrator) PrepareChildren(ctx context.Context, parent *task.State, config *task.Config) error {
    // Parallel-specific: all children prepared at once
    children := make([]ChildConfig, len(config.Tasks))
    for i, taskCfg := range config.Tasks {
        children[i] = ChildConfig{
            Config:   taskCfg,
            Index:    i,
            ParentID: parent.TaskExecID,
        }
    }

    return o.childPreparer.StoreChildren(ctx, parent.TaskExecID, children)
}

func (o *Orchestrator) CalculateStatus(ctx context.Context, parentID core.ID) (core.StatusType, error) {
    // Parallel-specific status calculation based on strategy
    children, err := o.TaskRepo.GetChildStates(ctx, parentID)
    if err != nil {
        return core.StatusUnknown, err
    }

    metadata, err := o.GetChildrenMetadata(ctx, parentID)
    if err != nil {
        return core.StatusUnknown, err
    }

    strategy := task.ParallelStrategy(metadata.CustomFields["strategy"].(string))
    return o.statusCalc.Calculate(children, strategy), nil
}
```

#### Collection Task Orchestrator

```go
// engine/task2/collection/orchestrator.go
package collection

type Orchestrator struct {
    *shared.BaseOrchestrator
    itemExpander *ItemExpander
    itemFilter   *ItemFilter
    childBuilder *ChildBuilder
}

func (o *Orchestrator) PrepareChildren(ctx context.Context, parent *task.State, config *task.Config) error {
    // Collection-specific: expand and filter items
    items, err := o.itemExpander.ExpandItems(ctx, config.CollectionConfig, parent)
    if err != nil {
        return fmt.Errorf("failed to expand collection items: %w", err)
    }

    filtered, err := o.itemFilter.FilterItems(ctx, items, config.CollectionConfig)
    if err != nil {
        return fmt.Errorf("failed to filter items: %w", err)
    }

    // Build child configs from filtered items
    children, err := o.childBuilder.BuildChildren(ctx, filtered, config, parent)
    if err != nil {
        return fmt.Errorf("failed to build child configs: %w", err)
    }

    return o.Storage.Store(ctx, o.childrenKey(parent.TaskExecID), children)
}
```

#### Wait Task Orchestrator

```go
// engine/task2/wait/orchestrator.go
package wait

type Orchestrator struct {
    *shared.BaseOrchestrator
    signalValidator *SignalValidator
}

func (o *Orchestrator) ValidateSignal(ctx context.Context, state *task.State, signal interfaces.Signal) error {
    // Wait-specific signal validation
    config, err := o.LoadTaskConfig(ctx, state.TaskExecID)
    if err != nil {
        return err
    }

    if config.WaitFor != signal.Name {
        return fmt.Errorf("task waiting for '%s', got '%s'", config.WaitFor, signal.Name)
    }

    return o.signalValidator.Validate(signal, config.WaitConfig)
}

func (o *Orchestrator) ProcessSignal(ctx context.Context, state *task.State, signal interfaces.Signal) (*task.State, error) {
    // Update state with signal data
    state.Status = core.StatusSuccess
    if state.Output == nil {
        state.Output = &core.Output{}
    }
    (*state.Output)["signal"] = signal.Payload
    (*state.Output)["signal_received_at"] = signal.Timestamp

    return state, o.TaskRepo.UpdateState(ctx, state)
}
```

### Factory Pattern

```go
// engine/task2/factory/orchestrator_factory.go
package factory

type OrchestratorFactory struct {
    orchestrators map[task.TaskType]func(*shared.OrchestratorContext) interfaces.TaskOrchestrator
}

func NewOrchestratorFactory() *OrchestratorFactory {
    f := &OrchestratorFactory{
        orchestrators: make(map[task.TaskType]func(*shared.OrchestratorContext) interfaces.TaskOrchestrator),
    }

    // Register built-in types
    f.Register(task.TaskTypeBasic, basic.NewOrchestrator)
    f.Register(task.TaskTypeParallel, parallel.NewOrchestrator)
    f.Register(task.TaskTypeCollection, collection.NewOrchestrator)
    f.Register(task.TaskTypeWait, wait.NewOrchestrator)
    // ... register other types

    return f
}

func (f *OrchestratorFactory) Create(taskType task.TaskType, ctx *shared.OrchestratorContext) (interfaces.TaskOrchestrator, error) {
    constructor, exists := f.orchestrators[taskType]
    if !exists {
        return nil, fmt.Errorf("no orchestrator registered for task type: %s", taskType)
    }
    return constructor(ctx), nil
}

func (f *OrchestratorFactory) Register(taskType task.TaskType, constructor func(*shared.OrchestratorContext) interfaces.TaskOrchestrator) {
    f.orchestrators[taskType] = constructor
}
```

## Integration Points

### Activity Layer Integration

Replace type-specific activities with generic orchestrator-based activity:

```go
// engine/task/activities/create_state.go
type CreateTaskState struct {
    factory      *factory.OrchestratorFactory
    orchContext  *shared.OrchestratorContext
}

func (a *CreateTaskState) Run(ctx context.Context, input *CreateTaskStateInput) (*task.State, error) {
    // Get appropriate orchestrator from factory
    orchestrator, err := a.factory.Create(input.TaskConfig.Type, a.orchContext)
    if err != nil {
        return nil, err
    }

    // Delegate to orchestrator
    return orchestrator.CreateState(ctx, interfaces.CreateStateInput{
        WorkflowState:  input.WorkflowState,
        WorkflowConfig: input.WorkflowConfig,
        TaskConfig:     input.TaskConfig,
    })
}
```

### Use Case Layer Updates

Remove type-specific logic from use cases:

```go
// engine/task/uc/create_child.go - REMOVE this entire use case
// Logic moves into respective orchestrators
```

## Testing Approach

### Unit Tests

Each orchestrator tested independently:

```go
// engine/task2/parallel/orchestrator_test.go
func TestParallelOrchestrator_CreateState(t *testing.T) {
    // Test parallel-specific state creation
}

func TestParallelOrchestrator_PrepareChildren(t *testing.T) {
    // Test child preparation logic
}

func TestParallelOrchestrator_CalculateStatus(t *testing.T) {
    // Test status aggregation strategies
}
```

### Integration Tests

Test orchestrator factory and integration:

```go
// engine/task2/factory/integration_test.go
func TestOrchestratorFactory_AllTypes(t *testing.T) {
    // Verify all task types can be created and executed
}
```

## Migration Strategy

### Phase 1: Create New Structure (Days 1-2)

1. Define all interfaces
2. Implement shared base orchestrator
3. Create factory pattern
4. Implement basic task orchestrator as proof

### Phase 2: Implement Simple Types (Days 3-4)

1. Implement wait task orchestrator
2. Implement signal task orchestrator
3. Create comprehensive tests
4. Validate pattern works

### Phase 3: Implement Complex Types (Days 5-7)

1. Implement parallel orchestrator with child management
2. Implement collection orchestrator with item processing
3. Implement composite orchestrator
4. Test parent-child relationships

### Phase 4: Integration (Days 8-9)

1. Update activity layer to use orchestrators
2. Create adapter for gradual migration
3. Update Temporal workflow activities
4. Remove old use cases

### Phase 5: Cleanup (Day 10)

1. Remove old services (ConfigManager, etc.)
2. Remove type-specific activities
3. Update documentation
4. Performance validation

## Standards Compliance

- ✅ **SOLID Principles**: Each orchestrator has single responsibility
- ✅ **Interface Segregation**: Optional interfaces for optional features
- ✅ **Open/Closed**: New types added without modifying existing code
- ✅ **Dependency Inversion**: Orchestrators depend on interfaces
- ✅ **Clean Architecture**: Clear boundaries and dependencies
- ✅ **Go Standards**: Follows project coding standards

## Performance Considerations

1. **No Performance Degradation**: Direct method calls, no reflection
2. **Efficient Storage**: Task-specific metadata storage patterns
3. **Optimized Queries**: Each orchestrator can optimize its queries
4. **Caching**: Orchestrators can implement appropriate caching

## Success Metrics

1. **Zero Type Switches**: No switch statements on task type
2. **Independent Tests**: Each orchestrator fully testable alone
3. **Clean Dependencies**: No circular dependencies
4. **Extensibility**: New task type added without touching existing code
5. **Performance**: Execution time within 5% of current system
