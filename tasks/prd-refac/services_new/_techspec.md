# TaskResponder & ConfigManager Modular Refactor - Technical Specification

## Executive Summary

This specification details the progressive refactor of TaskResponder (732 LOC) and ConfigManager (493 LOC) into clean architecture components following SOLID principles. ConfigManager is separated into domain logic (CollectionExpander) and infrastructure concerns (TaskConfigRepository), while TaskResponder becomes modular response handlers. This eliminates 1,225 LOC of monolithic code while maintaining system stability.

## System Architecture

### Current State Analysis

**TaskResponder Violations (732 LOC, 22 methods):**

- **SRP Violation**: Single class handles all task types (basic, parallel, collection, composite, etc.)
- **Good Pattern**: Uses delegation - HandleCollection() calls HandleMainTask() showing composition
- **Integration**: Called from 8 Activities methods (ExecuteBasicTask, GetParallelResponse, etc.)
- **Already uses task2**: Lines 36-39 show internal usage of task2 normalizers

**ConfigManager Mixed Responsibilities (493 LOC, 18 methods):**

- **Domain Logic**: PrepareCollectionConfigs (lines 137-210) - complex item expansion, filtering, templating
- **Infrastructure Logic**: PrepareParallelConfigs/PrepareCompositeConfigs - simple metadata storage
- **ISP Violation**: Type-specific methods violate Interface Segregation Principle
- **Integration**: Called from 6 Activities methods (CreateParallelState, CreateCollectionState, etc.)

### Target Architecture - Clean Architecture Separation

**Domain Layer:**

```
engine/task2/
├── collection/
│   ├── expander.go            # (new) - CollectionExpander domain service
│   └── response_handler.go    # (new) - Collection response logic
├── [basic|parallel|composite|router|wait|signal|aggregate]/
│   └── response_handler.go    # (new) - Task-specific response logic
├── core/
│   └── task_config_repository.go # (new) - Infrastructure service
└── shared/
    ├── interfaces.go           # Handler interfaces
    └── base_response_handler.go # Common response logic
```

## Implementation Design

This section has been moved to detailed task files for implementation. Key interfaces and patterns are defined in:

- **Task 1**: Core interfaces and shared components
- **Task 2**: CollectionExpander domain service implementation
- **Task 3**: TaskConfigRepository infrastructure service
- **Task 4-5**: Response handler foundation and task-specific implementations
- **Task 6**: Factory integration patterns

Refer to individual task files for complete implementation details and code examples.

## Integration Points

### Activities.go Updates

Integration patterns and migration strategies are detailed in:

- **Task 9**: Engine Integration - Complete integration patterns with Activities.go updates
- **Task 10**: Legacy Cleanup - Final removal and dependency updates

This approach ensures the techspec remains focused on architecture while task files contain specific implementation patterns.

## Impact Analysis

### Files Requiring Updates

Complete impact analysis including file changes, data model considerations, and migration strategies are detailed in:

- **Task 9**: Engine Integration - Files requiring updates and integration patterns
- **Task 10**: Legacy Cleanup - Removed files and cleanup tasks

### Data Model Impact

- **No database schema changes**
- **No API contract changes**
- **Configuration structures unchanged**
- **Metadata serialization format preserved**

## Testing Approach

Comprehensive testing strategies are detailed in dedicated task files:

- **Task 7**: Comprehensive Testing Suite - Unit tests, integration tests
- **Task 8**: Behavior Validation & Golden Master Tests - Regression prevention and behavior parity validation

Testing includes:

- **Golden Master Tests**: Capture current behavior for regression detection
- **End-to-End Tests**: Validate complete workflows still function
- **Unit Tests**: Component-level testing with >70% coverage requirement
- **Integration Tests**: Cross-component interaction validation

## Development Sequencing

The progressive refactor follows a 10-task implementation sequence detailed in individual task files:

**Phase 1: Foundation (Tasks 1-3)**

- Task 1: Shared Interfaces & Components
- Task 2: Collection Processing Domain Service
- Task 3: Task Configuration Repository

**Phase 2: Response Handlers (Tasks 4-6)**

- Task 4: Response Handler Foundation
- Task 5: Task-Specific Response Handlers
- Task 6: Extended Factory Integration

**Phase 3: Validation & Integration (Tasks 7-10)**

- Task 7: Comprehensive Testing Suite
- Task 8: Behavior Validation & Golden Master Tests
- Task 9: Engine Integration
- Task 10: Legacy Cleanup

Each task contains detailed implementation requirements, dependencies, success criteria, and testing requirements. This approach ensures systematic progress with validation at each step.

## Monitoring & Observability

### Key Metrics

- **Handler Creation Time**: Track factory.CreateResponseHandler latency
- **Repository Operations**: Track metadata storage/retrieval times
- **Error Rates**: Monitor failures by handler type and operation

### Logging Strategy

```go
log.Debug("Creating response handler",
    "task_type", taskType,
    "handler_type", reflect.TypeOf(handler))

log.Info("Collection expansion complete",
    "item_count", result.ItemCount,
    "skipped_count", result.SkippedCount,
    "duration_ms", duration.Milliseconds())
```

## Technical Considerations

### Clean Architecture Compliance

- **Domain Layer**: CollectionExpander contains business rules, no external dependencies
- **Infrastructure Layer**: TaskConfigRepository handles persistence, depends on domain
- **Application Layer**: Activities orchestrate domain services and infrastructure
- **Dependency Rule**: Dependencies point inward toward domain

### Implementation Considerations

- **Handler Creation**: Per-request creation adds minimal overhead
- **Domain Service**: Collection expansion behavior equivalent to current implementation
- **Repository Pattern**: Minimal overhead for simple storage operations
- **Memory Usage**: No significant increase in memory footprint

### Error Handling Strategy

- **Domain Errors**: CollectionExpander returns domain-specific errors
- **Infrastructure Errors**: Repository wraps storage errors with context
- **Handler Errors**: Response handlers preserve existing error semantics
- **Backward Compatibility**: Error messages and types remain unchanged

### Security & Standards Compliance

- **No new security surface**: Same permissions as existing services
- **SOLID Principles**: Each component has single responsibility
- **Clean Architecture**: Proper layer separation and dependency flow
- **Go Standards**: Follows established Go idioms and project conventions
