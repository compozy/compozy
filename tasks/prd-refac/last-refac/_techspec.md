# Activities and Use Cases Refactoring Plan

**Date**: January 3, 2025  
**Status**: APPROVED FOR IMPLEMENTATION  
**Criticality**: HIGH - Core application functionality  
**Estimated Effort**: 1-2 weeks  
**Approach**: GREENFIELD - No backwards compatibility required

## Executive Summary

This document outlines the comprehensive plan to refactor activities and use cases from their current centralized location (`engine/task/activities` and `engine/task/uc`) into task-type-specific folders within the `engine/task2` structure. This refactoring follows the established pattern in task2 where components are organized by task type rather than by component type.

### Key Decision: Clean Greenfield Approach

Since we're in the dev/alpha phase with no production users, we can implement a clean architectural design without backwards compatibility constraints. This allows us to prioritize code quality, simplicity, and maintainability over migration complexity.

## Current State Analysis

### Activities Structure (23 files)

- **Task-Specific**: 12 files handling basic, collection, parallel, composite, router, signal, wait, memory, and aggregate tasks
- **Shared Components**: 11 files including exec_subtask.go, response_helpers.go, response_converter.go, and configuration loaders
- **Integration Points**: Already integrated with task2 through Factory pattern

### Use Cases Structure (13 files)

- **Core Shared**: load_data.go, norm_config.go, handle_resp.go, create_state.go
- **Task-Specific**: exec_task.go (basic), exec_memory_operation.go (memory), process_wait_signal.go (wait), create_child.go (parent tasks)
- **Missing**: No specific use cases for router, signal, and aggregate tasks

### Dependencies

- 13 files in worker/executors import activities
- Activities depend on task2.Factory, shared.ContextBuilder, shared.ResponseInput
- Use cases depend on runtime.Manager, task.Repository, workflow.Repository

## Proposed Architecture

### Folder Structure

```
engine/task2/
├── contracts/
│   └── activities/                    (NEW)
│       ├── interfaces.go              # Activity interfaces
│       ├── factory.go                 # ActivityFactory interface
│       └── types.go                   # Common activity types
│
├── shared/
│   ├── activities/                    (NEW)
│   │   ├── base.go                   # BaseActivity struct
│   │   ├── subtask.go                # exec_subtask.go (parent shared)
│   │   ├── converter.go              # response_converter.go
│   │   ├── config_loader.go          # load_config.go strategies
│   │   └── helpers/
│   │       ├── response.go           # response_helpers.go
│   │       ├── progress.go           # get_progress.go
│   │       └── children.go           # list_children.go
│   └── uc/                           (NEW)
│       ├── base.go                   # BaseUseCase struct
│       ├── state.go                  # create_state.go
│       ├── response.go               # handle_resp.go
│       ├── config.go                 # norm_config.go, load_data.go
│       └── repository.go             # Common repository operations
│
├── basic/
│   ├── normalizer.go                 (existing)
│   ├── context_builder.go            (existing)
│   ├── response_handler.go           (existing)
│   ├── activities/                   (NEW)
│   │   └── execute.go               # exec_basic.go
│   └── uc/                          (NEW)
│       ├── execute.go               # exec_task.go
│       └── memory_resolver.go       # memory_resolver.go
│
├── collection/
│   ├── [existing files...]
│   ├── activities/                   (NEW)
│   │   ├── create_state.go          # collection_state.go
│   │   └── response.go              # collection_resp.go
│   └── uc/                          (NEW)
│       └── create_children.go       # Part of create_child.go
│
├── parallel/
│   ├── [existing files...]
│   ├── activities/                   (NEW)
│   │   ├── create_state.go          # parallel_state.go
│   │   └── response.go              # parallel_resp.go
│   └── uc/                          (NEW)
│       └── create_children.go       # Part of create_child.go
│
├── composite/
│   ├── [existing files...]
│   ├── activities/                   (NEW)
│   │   ├── create_state.go          # composite_state.go
│   │   └── response.go              # composite_resp.go
│   └── uc/                          (NEW)
│       └── create_children.go       # Part of create_child.go
│
├── router/
│   ├── [existing files...]
│   └── activities/                   (NEW)
│       └── execute.go               # exec_router.go
│
├── signal/
│   ├── [existing files...]
│   └── activities/                   (NEW)
│       └── execute.go               # exec_signal.go
│
├── wait/
│   ├── [existing files...]
│   ├── activities/                   (NEW)
│   │   ├── execute.go               # exec_wait.go
│   │   ├── helpers.go               # wait_helpers.go
│   │   └── processor.go             # wait_processor.go
│   └── uc/                          (NEW)
│       └── process_signal.go        # process_wait_signal.go
│
├── memory/
│   ├── [existing files...]
│   ├── activities/                   (NEW)
│   │   └── execute.go               # exec_memory.go
│   └── uc/                          (NEW)
│       └── operations.go            # exec_memory_operation.go
│
└── aggregate/
    ├── [existing files...]
    └── activities/                   (NEW)
        └── execute.go               # exec_aggregate.go
```

### Key Architectural Decisions

1. **Extension Pattern**: Add activities/ and uc/ subdirectories to existing task type folders
2. **Shared Module**: Create shared/activities and shared/uc for cross-cutting concerns
3. **Parent Task Abstraction**: Shared logic for collection/parallel/composite in shared/activities
4. **Interface Contracts**: Define interfaces in contracts/activities to prevent circular dependencies
5. **Factory Pattern**: Implement ActivityFactory for consistent creation and dependency injection

## Implementation Plan (Greenfield)

### Phase 1: Design & Structure (Days 1-2)

#### Tasks:

1. Create contracts/activities package with clean interfaces
2. Design optimal folder structure without legacy constraints
3. Set up base structures in shared/activities and shared/uc
4. Plan direct implementation approach

#### Deliverables:

- Clean interface definitions
- Optimized folder structure
- Base classes implemented

### Phase 2: Implementation (Days 3-7)

#### Approach:

- Implement all task types in parallel
- No migration needed - direct implementation in new structure
- Focus on clean code and optimal design

#### Implementation Order:

1. **Shared Components**: Base classes and utilities
2. **Simple Tasks**: basic, signal, wait, memory, aggregate, router
3. **Parent Tasks**: collection, parallel, composite (with shared parent module)

#### Process:

1. Create new folder structure for each task type
2. Implement activities/use cases directly in new locations
3. Use clean interfaces without legacy compatibility
4. Update all executor imports in one pass
5. Implement comprehensive tests

### Phase 3: Integration & Cleanup (Days 8-10)

#### Tasks:

1. Update all executor imports (13 files) in single commit
2. Remove old activity/uc folders completely
3. Update Temporal activity registration with new paths
4. Run comprehensive test suite
5. Performance validation

### Phase 4: Documentation & Polish (Days 11-12)

#### Tasks:

1. Update all documentation
2. Create architecture diagrams
3. Code review and refinements
4. Final testing pass

## Risk Mitigation Strategies (Simplified for Greenfield)

### 1. Clean Architecture

- **Risk**: Circular dependencies
- **Mitigation**:
    - Use interfaces in contracts package
    - Clear layer boundaries
    - Automated dependency checks
    - Clean separation of concerns

### 2. Code Quality

- **Risk**: Regression in functionality
- **Mitigation**:
    - Comprehensive test coverage from start
    - Parallel test runs during development
    - Code reviews at each phase
    - Performance benchmarks

### 3. Development Velocity

- **Risk**: Blocking other development
- **Mitigation**:
    - Feature branch approach
    - Daily integration tests
    - Clear communication of changes
    - Modular implementation

## Success Criteria

### Functional Requirements

- [ ] All tests pass with new structure
- [ ] Clean API design without legacy constraints
- [ ] All task types execute correctly
- [ ] Simplified codebase structure

### Non-Functional Requirements

- [ ] Improved performance (target: 10-20% better)
- [ ] Reduced memory footprint
- [ ] Clean dependency graph (no cycles)
- [ ] 90%+ code coverage

### Quality Metrics

- [ ] Zero coupling between task types
- [ ] High cohesion within each task type
- [ ] Intuitive folder structure
- [ ] Reduced code complexity scores

## Technical Considerations

### Interface Design

```go
// contracts/activities/interfaces.go
package activities

import (
    "context"
    "github.com/compozy/compozy/engine/task"
)

type Activity interface {
    Name() string
    Execute(ctx context.Context, input interface{}) (interface{}, error)
}

type TaskActivity interface {
    Activity
    TaskType() task.Type
}

type ActivityFactory interface {
    CreateActivity(name string) (Activity, error)
    RegisterActivity(activity Activity) error
    ListActivities() []string
}
```

### Factory Implementation

```go
// contracts/activities/factory.go
package activities

type Dependencies struct {
    TemplateEngine *tplengine.TemplateEngine
    TaskRepo       task.Repository
    WorkflowRepo   workflow.Repository
    ConfigStore    services.ConfigStore
    Task2Factory   task2.Factory
}

type TaskActivityFactory interface {
    CreateActivities(deps Dependencies) ([]Activity, error)
}
```

### Implementation Example (Basic Task)

```go
// task2/basic/activities/execute.go
package activities

import (
    "github.com/compozy/compozy/engine/task2/contracts/activities"
    "github.com/compozy/compozy/engine/task2/shared/activities"
)

type ExecuteBasic struct {
    activities.BaseActivity
    runtime    *runtime.Manager
    processor  processors.Processor
}

func NewExecuteBasic(deps activities.Dependencies) activities.Activity {
    return &ExecuteBasic{
        BaseActivity: activities.NewBaseActivity("ExecuteBasic", task.TaskTypeBasic),
        runtime:      deps.Runtime,
        processor:    deps.ProcessorFactory.Create(task.TaskTypeBasic),
    }
}

func (a *ExecuteBasic) Execute(ctx context.Context, input *ExecuteBasicInput) (*ExecuteBasicOutput, error) {
    // Clean implementation without legacy compatibility
    return a.processor.Execute(ctx, input.TaskConfig)
}
```

## Development Process

### Branch Strategy:

1. Create feature branch: `refactor/task2-activities-uc`
2. Implement in isolated environment
3. Daily rebases from main
4. Merge when complete and tested

### Quality Gates:

- All tests must pass
- Code coverage >= 90%
- Performance benchmarks pass
- Code review approval
- No linting errors

## Long-term Benefits

1. **Clean Architecture**: Pure task-type separation without legacy compromises
2. **Performance**: Optimized code paths without compatibility layers
3. **Simplicity**: Reduced complexity from removing backwards compatibility
4. **Maintainability**: Clear, intuitive structure for developers
5. **Extensibility**: New task types have a clear pattern to follow
6. **Testing**: Isolated test suites per task type

## Greenfield Advantages

By taking a greenfield approach in the alpha phase:

- **No technical debt** from compatibility requirements
- **Optimal design** decisions without constraints
- **Faster implementation** (1-2 weeks vs 3-4 weeks)
- **Cleaner codebase** for future development
- **Better performance** without abstraction overhead

## Conclusion

This greenfield refactoring leverages our alpha phase status to implement an ideal architecture without backwards compatibility constraints. The result will be a cleaner, more maintainable codebase that serves as a solid foundation for the product's future growth.

The simplified approach reduces implementation time by 50% while delivering a superior architectural outcome.

---

**Approval**: **\*\***\_\_\_**\*\***  
**Date**: **\*\***\_\_\_**\*\***  
**Notes**: Greenfield approach approved for alpha phase development. No backwards compatibility required.
