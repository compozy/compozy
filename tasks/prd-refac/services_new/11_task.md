# Task 11: Centralize NormalizerFactory and TaskNormalizer Interfaces

STATUS: ✅ DONE

## Problem Statement

The `engine/task2` package has multiple duplicate definitions of `NormalizerFactory` and `TaskNormalizer` interfaces across different sub-packages:

1. `core/interfaces.go` - Minimal interfaces without Type() method
2. `task2/normalizer.go` - TaskNormalizer with Type() method
3. `task2/interfaces.go` - Factory interface that creates normalizers
4. `shared/base_subtask_normalizer.go` - Duplicate interfaces to avoid import cycles

This duplication exists because of cyclic dependency issues where `shared` package needs to create normalizers but cannot import the parent `task2` package.

## Root Cause Analysis

### Dependency Graph

```
task2 → shared → task2 (blocked by Go's import rules)

Specific issue:
- task2/parallel → shared (uses BaseSubTaskNormalizer)
- task2/composite → shared (uses BaseSubTaskNormalizer)
- shared/BaseSubTaskNormalizer needs Factory from task2
- shared cannot import task2 (would create cycle)
```

### Current Workaround Issues

- **Maintenance burden**: Interfaces must be kept synchronized across 3-4 locations
- **Type safety issues**: Relies on structural compatibility rather than explicit interfaces
- **Developer confusion**: Multiple versions of the same interface
- **Adapter complexity**: Requires `factoryAdapter` to bridge duplicate interfaces

## Proposed Solution

### Architecture: Dependency Inversion with Contracts Package

Create a new `contracts` package that defines all shared interfaces with zero dependencies:

```
engine/task2/contracts/
├── normalizer.go      # TaskNormalizer interface
├── factory.go         # NormalizerFactory interface
└── response.go        # TaskResponseHandler interface (future)
```

### New Dependency Flow (No Cycles)

```
contracts (no dependencies)
    ↑
    ├── core
    ├── shared
    ├── task2
    └── all task-specific packages
```

## Implementation Plan

### Phase 1: Create Contracts Package

1. **Create directory structure**

    ```bash
    mkdir -p engine/task2/contracts
    ```

2. **Define unified interfaces**

    ```go
    // engine/task2/contracts/normalizer.go
    package contracts

    import (
        "github.com/compozy/compozy/engine/task"
        "github.com/compozy/compozy/engine/task2/shared"
    )

    // TaskNormalizer defines the contract for task-specific normalization
    type TaskNormalizer interface {
        // Normalize applies task-specific normalization rules
        Normalize(config *task.Config, ctx *shared.NormalizationContext) error
        // Type returns the task type this normalizer handles
        Type() task.Type
    }
    ```

    ```go
    // engine/task2/contracts/factory.go
    package contracts

    import "github.com/compozy/compozy/engine/task"

    // NormalizerFactory defines the contract for creating task normalizers
    type NormalizerFactory interface {
        // CreateNormalizer creates a normalizer for the given task type
        CreateNormalizer(taskType task.Type) (TaskNormalizer, error)
    }
    ```

### Phase 2: Update Core Package

1. **Remove duplicate interfaces from `core/interfaces.go`**
2. **Import from contracts package**
3. **Update any type references**

### Phase 3: Update Shared Package

1. **Remove duplicate interfaces from `shared/base_subtask_normalizer.go`**
    - Remove `TaskNormalizerInterface`
    - Remove `NormalizerFactoryInterface`
2. **Update imports to use contracts**
    ```go
    import "github.com/compozy/compozy/engine/task2/contracts"
    ```
3. **Update BaseSubTaskNormalizer to use contracts interfaces**
    ```go
    type BaseSubTaskNormalizer struct {
        templateEngine    *tplengine.TemplateEngine
        contextBuilder    *ContextBuilder
        normalizerFactory contracts.NormalizerFactory
        taskType          task.Type
        taskTypeName      string
    }
    ```

### Phase 4: Update Task2 Package

1. **Remove TaskNormalizer from `task2/normalizer.go`**
2. **Update Factory interface in `task2/interfaces.go`**
    ```go
    type Factory interface {
        contracts.NormalizerFactory
        // ... other methods
    }
    ```
3. **Remove factoryAdapter** - no longer needed

### Phase 5: Update All Implementations

1. **Update imports in all normalizer implementations**:

    - `basic/normalizer.go`
    - `parallel/normalizer.go`
    - `composite/normalizer.go`
    - `collection/normalizer.go`
    - `router/normalizer.go`
    - `aggregate/normalizer.go`
    - `signal/normalizer.go`
    - `wait/normalizer.go`

2. **Ensure all implement contracts.TaskNormalizer**

### Phase 6: Testing and Validation

1. **Run unit tests for each package**

    ```bash
    go test ./engine/task2/...
    ```

2. **Run integration tests**

    ```bash
    make test
    ```

3. **Verify no import cycles**
    ```bash
    go mod graph | grep task2
    ```

## Benefits

1. **Single Source of Truth**: One definition per interface in contracts package
2. **No Cyclic Dependencies**: contracts package has zero dependencies
3. **Type Safety**: Explicit interface implementation checking
4. **Maintainability**: No duplicate code to keep synchronized
5. **Clarity**: Clear architectural boundaries
6. **Extensibility**: Easy to add new interfaces to contracts

## Risks and Mitigation

1. **Risk**: Large number of files to update

    - **Mitigation**: Update in phases, test after each phase

2. **Risk**: Potential for missed references

    - **Mitigation**: Use grep/IDE to find all occurrences

3. **Risk**: Breaking existing functionality
    - **Mitigation**: Comprehensive test coverage before refactoring

## Success Criteria

1. All duplicate interface definitions removed
2. No import cycles in the codebase
3. All tests passing
4. No need for adapter code
5. Clear dependency hierarchy

## Timeline Estimate

- Phase 1-2: 1 hour (create contracts, update core)
- Phase 3-4: 2 hours (update shared and task2)
- Phase 5: 2 hours (update all implementations)
- Phase 6: 1 hour (testing and validation)

**Total: ~6 hours**

## Alternative Approaches Considered

1. **Move everything to core package**

    - Rejected: Would make core too heavy with dependencies

2. **Use interface{} and type assertions**

    - Rejected: Loss of type safety

3. **Keep duplicates but use code generation**
    - Rejected: Adds complexity, doesn't solve root issue

## Conclusion

The contracts package approach provides a clean, maintainable solution that follows Go best practices and SOLID principles. It eliminates code duplication while maintaining type safety and avoiding import cycles.
