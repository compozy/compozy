# Task Services Migration Plan V2 - Task-Specific Architecture

## Overview

This document outlines a revised migration plan that properly architects task-specific service implementations rather than simply moving shared services. Each task type will have its own implementation of the services it needs, following Interface Segregation and Single Responsibility principles.

## Current Problems

1. **Monolithic Services**: ConfigManager has methods for all task types (PrepareParallelConfigs, PrepareCollectionConfigs, etc.)
2. **Shared State Management**: All task types use the same state creation logic
3. **Generic Status Updates**: Parent status updates don't account for task-specific strategies
4. **Coupling**: Services know about all task types instead of focusing on their domain

## Proposed Architecture

### Core Interfaces

```go
// engine/task2/interfaces/orchestrator.go
type TaskOrchestrator interface {
    // Core orchestration for a task type
    CreateState(ctx context.Context, input CreateStateInput) (*task.State, error)
    HandleResponse(ctx context.Context, input HandleResponseInput) (*task.Response, error)
}

// engine/task2/interfaces/config_manager.go
type ConfigManager interface {
    // Prepare any configuration needed before execution
    PrepareConfig(ctx context.Context, taskID core.ID, config *task.Config) error
    // Load prepared configuration
    LoadConfig(ctx context.Context, taskID core.ID) (*task.Config, error)
}

// engine/task2/interfaces/child_manager.go
type ChildTaskManager interface {
    // Prepare child tasks (for container types)
    PrepareChildren(ctx context.Context, parent *task.State, config *task.Config) error
    // Create child task states
    CreateChildren(ctx context.Context, parentID core.ID) ([]*task.State, error)
}

// engine/task2/interfaces/status_manager.go
type StatusManager interface {
    // Update status based on task-specific logic
    UpdateStatus(ctx context.Context, state *task.State) error
    // Calculate parent status from children (for container types)
    CalculateParentStatus(ctx context.Context, parentID core.ID) (core.StatusType, error)
}

// engine/task2/interfaces/signal_handler.go
type SignalHandler interface {
    // Handle incoming signals (for wait/signal tasks)
    HandleSignal(ctx context.Context, taskID core.ID, signal Signal) error
    // Validate if signal can be processed
    ValidateSignal(ctx context.Context, taskID core.ID, signal Signal) error
}
```

### Task-Specific Implementations

#### Parallel Task

```
engine/task2/parallel/
├── orchestrator.go      # Implements TaskOrchestrator
├── config_manager.go    # Stores child configs for parallel execution
├── child_manager.go     # Creates all children at once
└── status_manager.go    # Aggregates status based on strategy (wait_all, wait_any)
```

#### Collection Task

```
engine/task2/collection/
├── orchestrator.go      # Implements TaskOrchestrator
├── config_manager.go    # Expands and filters collection items
├── child_manager.go     # Creates children from filtered items
├── item_processor.go    # Collection-specific item handling
└── status_manager.go    # Tracks progress with item counts
```

#### Wait Task

```
engine/task2/wait/
├── orchestrator.go      # Implements TaskOrchestrator
├── signal_handler.go    # Processes wait_for signals
└── status_manager.go    # Updates status on signal receipt
```

#### Basic Task

```
engine/task2/basic/
├── orchestrator.go      # Simple implementation, no special handling
└── config_manager.go    # Basic config storage/retrieval
```

### Shared Components

```
engine/task2/shared/
├── base_orchestrator.go # Common orchestration logic
├── base_config.go       # Common config operations
├── base_status.go       # Common status calculations
└── storage/             # Storage interfaces and implementations
    ├── interfaces.go
    └── redis_store.go
```

## Migration Strategy

### Phase 1: Define Interfaces and Base Components

1. Create `engine/task2/interfaces/` with all interface definitions
2. Implement shared base components in `engine/task2/shared/`
3. Set up storage abstraction layer

### Phase 2: Implement Task-Specific Orchestrators

Start with the simplest and build up:

1. **Basic Task** - Minimal implementation to validate approach
2. **Wait Task** - Add signal handling capability
3. **Parallel Task** - Add child management and status aggregation
4. **Collection Task** - Add item processing and filtering

### Phase 3: Integrate with Existing System

1. Create adapter layer to bridge old and new systems
2. Update task activities to use new orchestrators
3. Gradually migrate each task type
4. Remove old shared services once all types migrated

## Key Differences from V1

### V1 (Wrong Approach)

- Move shared services to new location
- Keep monolithic service design
- Maintain PrepareParallelConfigs, PrepareCollectionConfigs methods
- Services know about all task types

### V2 (Correct Approach)

- Each task type has its own service implementations
- Interfaces define contracts, not implementations
- No task-type-specific methods in interfaces
- Services only know about their specific task type
- Proper separation of concerns

## Implementation Benefits

1. **Extensibility**: New task types just implement the interfaces they need
2. **Maintainability**: Task logic is isolated and cohesive
3. **Testability**: Each implementation can be tested independently
4. **Performance**: No unnecessary checks or branches for task types
5. **Clarity**: Clear where to find/modify task-specific behavior

## Example: Parallel Task Implementation

```go
// engine/task2/parallel/orchestrator.go
type ParallelOrchestrator struct {
    configMgr  ConfigManager
    childMgr   ChildTaskManager
    statusMgr  StatusManager
    taskRepo   task.Repository
}

func (o *ParallelOrchestrator) CreateState(ctx context.Context, input CreateStateInput) (*task.State, error) {
    // 1. Create parent state
    state := o.createParentState(input)

    // 2. Prepare child configurations
    if err := o.configMgr.PrepareConfig(ctx, state.ID, input.Config); err != nil {
        return nil, err
    }

    // 3. Store parent state
    if err := o.taskRepo.Create(ctx, state); err != nil {
        return nil, err
    }

    // 4. Prepare children for later creation
    if err := o.childMgr.PrepareChildren(ctx, state, input.Config); err != nil {
        return nil, err
    }

    return state, nil
}
```

## Success Criteria

1. Each task type has its own package with focused implementations
2. No shared service has task-type-specific methods
3. New task types can be added without modifying existing code
4. All tests pass with improved isolation
5. Performance maintained or improved
