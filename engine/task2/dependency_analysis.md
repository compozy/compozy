# Task2 Package Dependency Analysis

## Summary

The engine/task2 package has a cyclic dependency issue that causes interface duplication across multiple packages. The same `TaskNormalizer` interface is defined in 3 different places, and the factory interface is defined in 2 places, all to work around Go's restriction on circular imports.

## Cyclic Dependency Issues Identified

### 1. Core Dependency Graph

```
┌─────────────────────────────────────────────────────────────────┐
│                           task2                                   │
│  - Defines: TaskNormalizer interface                             │
│  - Defines: Factory interface                                    │
│  - Imports: ALL subpackages (aggregate, basic, collection, etc.) │
└─────────────────────────────────────────────────────────────────┘
                    │                           ▲
                    │ imports                   │ would need to import
                    ▼                           │ (but can't - cycle!)
┌─────────────────────────────────────────────────────────────────┐
│                         task2/shared                             │
│  - Cannot import task2 (would create cycle)                     │
│  - Defines: TaskNormalizerInterface (duplicate)                 │
│  - Defines: NormalizerFactoryInterface (duplicate)              │
│  - Used by: ALL task type packages                              │
└─────────────────────────────────────────────────────────────────┘
                    ▲                           ▲
                    │ imports                   │ imports
                    │                           │
┌───────────────────┴────────┐     ┌───────────┴─────────────────┐
│      task2/parallel        │     │      task2/composite        │
│  - Uses: BaseSubTask-      │     │  - Uses: BaseSubTask-       │
│    Normalizer              │     │    Normalizer               │
│  - Needs: Factory to       │     │  - Needs: Factory to        │
│    create sub-normalizers  │     │    create sub-normalizers   │
└────────────────────────────┘     └─────────────────────────────┘
```

### 2. The Cyclic Dependency Chain

1. **task2** → imports **task2/shared** (for shared types and utilities)
2. **task2** → imports **task2/parallel** and **task2/composite** (to register normalizers in factory)
3. **task2/parallel** and **task2/composite** → import **task2/shared** (for BaseSubTaskNormalizer)
4. **task2/shared** needs **task2.Factory** interface to create sub-normalizers recursively
5. BUT **task2/shared** cannot import **task2** because that would create a cycle!

### 3. Complete List of Duplicate Interfaces

#### TaskNormalizer Interface (defined 3 times):

1. **task2/normalizer.go**:

```go
type TaskNormalizer interface {
    Normalize(config *task.Config, ctx *shared.NormalizationContext) error
    Type() task.Type
}
```

2. **task2/shared/base_subtask_normalizer.go**:

```go
type TaskNormalizerInterface interface {
    Normalize(config *task.Config, ctx *NormalizationContext) error
    Type() task.Type
}
```

3. **task2/core/interfaces.go**:

```go
type TaskNormalizer interface {
    Normalize(config *task.Config, ctx *shared.NormalizationContext) error
    // Note: Missing Type() method - incomplete interface
}
```

#### Factory Interface (defined 2 times):

1. **task2/interfaces.go**:

```go
type Factory interface {
    CreateNormalizer(taskType task.Type) (TaskNormalizer, error)
    // ... plus many other methods
}
```

2. **task2/shared/base_subtask_normalizer.go**:

```go
type NormalizerFactoryInterface interface {
    CreateNormalizer(taskType task.Type) (TaskNormalizerInterface, error)
}
```

3. **task2/core/interfaces.go**:

```go
type NormalizerFactory interface {
    CreateNormalizer(taskType task.Type) (TaskNormalizer, error)
}
```

### 4. Why Interface Duplication Exists

#### In `task2/shared/base_subtask_normalizer.go`:

```go
// These are duplicates of task2 interfaces to avoid import cycles:
type TaskNormalizerInterface interface {
    Normalize(config *task.Config, ctx *NormalizationContext) error
    Type() task.Type
}

type NormalizerFactoryInterface interface {
    CreateNormalizer(taskType task.Type) (TaskNormalizerInterface, error)
}
```

#### In `task2/core/interfaces.go`:

```go
// Another duplicate of the same interface:
type TaskNormalizer interface {
    Normalize(config *task.Config, ctx *shared.NormalizationContext) error
}
```

### 4. The Root Cause

The fundamental issue is that:

- `BaseSubTaskNormalizer` (in shared package) needs to create normalizers for sub-tasks
- To create normalizers, it needs the factory
- The factory is defined in the task2 package
- But shared cannot import task2 without creating a cycle

### 5. Current Workaround

The code uses a `factoryAdapter` in `task2/factory.go` to bridge the interfaces:

```go
type factoryAdapter struct {
    factory Factory
}

func (a *factoryAdapter) CreateNormalizer(taskType task.Type) (shared.TaskNormalizerInterface, error) {
    normalizer, err := a.factory.CreateNormalizer(taskType)
    if err != nil {
        return nil, err
    }
    // Since both interfaces have the same methods, we can use a type assertion
    return normalizer, nil
}
```

This adapter allows the factory (which implements `task2.Factory`) to be passed to shared components that expect `shared.NormalizerFactoryInterface`.

### 6. Impact

1. **Interface Duplication**: Same interfaces defined in 3 places
2. **Type Safety Issues**: Relying on structural compatibility rather than explicit interface implementation
3. **Maintenance Burden**: Changes to interfaces must be synchronized across multiple definitions
4. **Confusion**: Developers must understand why there are multiple versions of the same interface

### 7. Where the Cycle Manifests

The cycle specifically occurs in the sub-task normalization flow:

```go
// In shared/base_subtask_normalizer.go:
func (n *BaseSubTaskNormalizer) normalizeSingleSubTask(...) error {
    // This line needs to create a normalizer for the sub-task type
    subNormalizer, err := n.normalizerFactory.CreateNormalizer(subTask.Type)
    // ...
}
```

This is needed because:

- Parallel tasks can contain any type of sub-tasks (basic, collection, even other parallel tasks)
- Composite tasks can also contain any type of sub-tasks
- The normalizer needs to recursively normalize these sub-tasks using the appropriate normalizer

### 8. Dependency Flow Example

```
1. User creates a parallel task with basic and collection sub-tasks
2. ParallelNormalizer.Normalize() is called
3. It delegates to BaseSubTaskNormalizer.Normalize()
4. BaseSubTaskNormalizer needs to normalize each sub-task:
   - For basic sub-task: needs to create BasicNormalizer
   - For collection sub-task: needs to create CollectionNormalizer
5. To create these normalizers, it needs the Factory
6. But Factory is in task2 package, which shared cannot import
```

### 9. Visual Representation of the Cycle

```
┌─────────────────────┐
│ task2/factory.go    │
│                     │
│ func NewFactory()   │──────creates──────┐
│ Factory {           │                   │
│   CreateNormalizer()│                   ▼
│ }                   │         ┌─────────────────────┐
└─────────────────────┘         │ ParallelNormalizer  │
         ▲                      │ CompositeNormalizer │
         │                      │                     │
         │ imports              │ embeds              │
         │                      ▼                     │
┌─────────────────────┐    ┌──────────────────────┐  │
│ task2 package       │    │ BaseSubTaskNormalizer│  │
│                     │    │                      │  │
│ Defines:            │    │ needs Factory to     │  │
│ - TaskNormalizer    │    │ create sub-          │  │
│ - Factory           │    │ normalizers          │  │
└─────────────────────┘    └──────────────────────┘  │
         ▲                           │                │
         │                           │                │
         │ would need to import      │ lives in      │
         │ (BUT CAN'T - CYCLE!)      ▼                │
         │                  ┌─────────────────────┐   │
         └──────────────────│ task2/shared       │───┘
                           │                     │
                           │ Defines duplicates: │
                           │ - TaskNormalizer-   │
                           │   Interface         │
                           │ - NormalizerFactory-│
                           │   Interface         │
                           └─────────────────────┘
```

### 10. Possible Solutions

1. **Move Factory to shared package**: But this would require moving all task type imports
2. **Use interface injection**: Define minimal interfaces where needed (current approach)
3. **Restructure packages**:
    - Option A: Move BaseSubTaskNormalizer to task2 package
    - Option B: Create a separate "factory" package that both can import
    - Option C: Use a registry pattern where normalizers register themselves
4. **Accept the duplication**: Document it clearly as a necessary evil (current approach)
