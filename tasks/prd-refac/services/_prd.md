# Task Services Architecture Refactoring PRD

## Overview

This PRD outlines the architectural refactoring needed to transform the current monolithic task services into a properly modularized, task-type-specific architecture. The current system violates SOLID principles with shared services containing type-specific methods and switch statements. The new architecture will have each task type own its complete orchestration lifecycle.

## Problem Statement

The current task services architecture has fundamental design flaws:

1. **Interface Segregation Violation**: Services like `ConfigManager` have type-specific methods (`PrepareParallelConfigs`, `PrepareCollectionConfigs`) that not all clients need
2. **Open/Closed Principle Violation**: Adding new task types requires modifying existing services (switch statements)
3. **Single Responsibility Violation**: Services handle multiple task types instead of focusing on one
4. **Unnecessary Coupling**: All task types depend on shared services even when they don't need all functionality

Example of the anti-pattern:

```go
// Bad: Switch on type
switch parentState.ExecutionType {
case task.ExecutionParallel:
    return uc.createParallelChildren(ctx, parentState)
case task.ExecutionCollection:
    return uc.createCollectionChildren(ctx, parentState)
// ...
}
```

## Goals

1. **Eliminate Type Switching**: Remove all switch statements on task type from services
2. **Task-Specific Ownership**: Each task type owns its complete orchestration logic
3. **Clean Interfaces**: Define minimal interfaces that make sense for all implementations
4. **Extensibility**: New task types can be added without modifying existing code
5. **Maintain Functionality**: All current features continue to work during and after migration

## User Stories

### As a Developer

- I want each task type's logic in one place so I can understand and modify it without affecting others
- I want to add new task types without modifying existing services
- I want clear interfaces that don't force me to implement unnecessary methods

### As a System Architect

- I want the architecture to follow SOLID principles for long-term maintainability
- I want clear separation of concerns between different task types
- I want the system to be extensible without modification

## Core Features

### 1. Task Orchestrator Interface

**What it does**: Defines the contract for task orchestration that each task type implements

**Why it's important**: Provides a common interface while allowing task-specific implementations

**Functional Requirements**:
1.1. Define minimal interface that all task types can reasonably implement
1.2. Support state creation, execution, and response handling
1.3. Allow optional interfaces for task-specific features

### 2. Task-Specific Orchestrators

**What it does**: Each task type has its own orchestrator implementation

**Why it's important**: Encapsulates all task-specific logic in one place

**Functional Requirements**:
2.1. Parallel orchestrator handles child creation and status aggregation
2.2. Collection orchestrator handles item expansion and filtering
2.3. Wait orchestrator handles signal validation and processing
2.4. Basic orchestrator provides simple execution without special features

### 3. Optional Capability Interfaces

**What it does**: Defines interfaces for capabilities that only some task types need

**Why it's important**: Allows task types to opt-in to features they need

**Functional Requirements**:
3.1. ChildTaskManager for tasks that create children (parallel, collection)
3.2. SignalHandler for tasks that process signals (wait, signal)
3.3. StatusAggregator for tasks that aggregate child status

### 4. Orchestrator Factory

**What it does**: Creates the appropriate orchestrator for a given task type

**Why it's important**: Centralizes orchestrator creation without coupling to implementations

**Functional Requirements**:
4.1. Map task types to their orchestrator implementations
4.2. Return appropriate orchestrator based on task configuration
4.3. Support registration of new task types

## Architecture Overview

### Current (Bad) Architecture

```
Services (Shared)
├── ConfigManager
│   ├── PrepareParallelConfigs()
│   ├── PrepareCollectionConfigs()
│   └── PrepareCompositeConfigs()
├── CreateChildTasks
│   └── Execute() with switch statement
└── ParentStatusUpdater
    └── Shared logic for all types
```

### New (Good) Architecture

```
engine/task2/
├── interfaces/
│   ├── orchestrator.go      # Core orchestration interface
│   ├── child_manager.go     # Optional: for container tasks
│   └── signal_handler.go    # Optional: for signal tasks
├── parallel/
│   └── orchestrator.go      # Complete parallel logic
├── collection/
│   └── orchestrator.go      # Complete collection logic
├── wait/
│   └── orchestrator.go      # Complete wait logic
├── basic/
│   └── orchestrator.go      # Simple task logic
└── factory/
    └── orchestrator_factory.go
```

## Non-Goals

1. **Changing External APIs**: Task YAML format remains unchanged
2. **Modifying Persistence**: Database schema stays the same
3. **Altering Workflow Engine**: Temporal/Cadence integration unchanged
4. **Feature Changes**: No new functionality, only architectural improvements

## Success Metrics

1. **Zero Type Switches**: No switch statements on task type in orchestration code
2. **Interface Compliance**: All interfaces follow Interface Segregation Principle
3. **Independent Modules**: Each task type can be modified without affecting others
4. **Test Isolation**: Task types can be tested independently
5. **Performance**: No degradation in execution speed

## Risks and Mitigations

### Risk 1: Breaking Existing Functionality

**Mitigation**: Incremental migration with comprehensive testing at each step

### Risk 2: Complex Migration Path

**Mitigation**: Create adapter layer to bridge old and new systems during transition

### Risk 3: Hidden Dependencies

**Mitigation**: Thorough analysis of current codebase before starting migration

## Migration Strategy

### Phase 1: Foundation (Week 1)

- Define interfaces
- Create factory pattern
- Implement basic orchestrator as proof of concept

### Phase 2: Simple Types (Week 2)

- Migrate basic and wait tasks
- These have minimal dependencies and prove the pattern

### Phase 3: Complex Types (Week 3)

- Migrate parallel and collection tasks
- These require child management and status aggregation

### Phase 4: Integration (Week 4)

- Update all integration points
- Remove old services
- Comprehensive testing

## Example Implementation

```go
// Good: Each task type owns its logic
type ParallelOrchestrator struct {
    repo      task.Repository
    storage   Storage
}

func (o *ParallelOrchestrator) CreateState(ctx context.Context, input CreateStateInput) (*task.State, error) {
    // Parallel-specific state creation
    state := o.createParallelState(input)

    // Parallel-specific child preparation
    children := o.prepareChildTasks(input.Config)

    // Store for later execution
    o.storage.StoreChildren(state.ID, children)

    return state, nil
}

// No type checking needed - this is inherently parallel-specific
```

## Open Questions

1. How do we handle shared functionality like storage access?
2. Should status aggregation strategies be pluggable?
3. How do we manage the transition period with both systems?
4. What's the best way to test the migration?

## Appendix

### Anti-Pattern Examples from Current Code

1. **ConfigManager with type-specific methods**:

    - PrepareParallelConfigs
    - PrepareCollectionConfigs
    - PrepareCompositeConfigs

2. **CreateChildTasks with switch statement**:

    - Switches on ExecutionType
    - Delegates to type-specific methods

3. **Shared services knowing about all types**:
    - ParentStatusUpdater handles all parent types
    - WaitTaskManager mixed with general task logic
