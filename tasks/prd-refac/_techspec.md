# Technical Specification: Normalizer Package Refactoring

## Executive Summary

This technical specification outlines the refactoring of the monolithic `pkg/normalizer/` package into a modular, task-type-specific architecture under `engine/task2/`. The refactoring addresses critical architectural violations (SRP, OCP) while preserving the package's strengths, particularly the well-designed `ContextBuilder` and template engine integration.

## Current State Analysis

### Architecture Problems

1. **Single Responsibility Violation**: The `pkg/normalizer/` package handles all task type normalization in one place
2. **Open/Closed Principle Violation**: Adding new task types requires modifying core normalizer logic
3. **Inconsistent Patterns**: Collection tasks use dedicated struct, parallel tasks use inline methods
4. **Tight Coupling**: Normalizer is deeply coupled to all task type configurations
5. **Maintenance Burden**: 676 lines for collection logic alone, multiplied across all task types

### Current Components

```
pkg/normalizer/
├── normalizer.go           # Main normalizer with type-specific branching
├── config.go               # ConfigNormalizer - orchestration & env merging
├── context.go              # ContextBuilder & NormalizationContext
├── collection.go           # CollectionNormalizer
├── collection_*.go         # Collection utilities (filter, converter, builder)
└── config_*.go             # Configuration builders
```

### Strengths to Preserve

1. **ContextBuilder**: Excellent design for building normalization contexts
2. **Template Engine Integration**: Well-implemented template processing
3. **Environment Merging**: Sophisticated workflow→task→component merging
4. **Collection Patterns**: Good example of task-specific normalization

## Proposed Architecture

### High-Level Structure

```
engine/task2/
├── normalizer.go           # Core interfaces
├── factory.go              # NormalizerFactory
├── orchestrator.go         # Main orchestration logic
├── shared/                 # Shared components
│   ├── context.go          # NormalizationContext & ContextBuilder
│   └── interfaces.go       # Common interfaces
├── core/                   # Core services
│   ├── config_normalizer.go    # Main orchestration
│   ├── output_transformer.go   # Output normalization
│   ├── env_merger.go           # Environment merging
│   ├── agent_normalizer.go     # Agent component normalization
│   ├── tool_normalizer.go      # Tool component normalization
│   ├── success_transition.go   # Success transition normalization
│   └── error_transition.go     # Error transition normalization
├── basic/                  # Basic task normalizer
│   └── normalizer.go
├── parallel/               # Parallel task normalizer
│   └── normalizer.go
├── collection/             # Collection task normalizer
│   ├── normalizer.go
│   ├── config_builder.go
│   ├── type_converter.go
│   └── filter_evaluator.go
├── router/                 # Router task normalizer
├── wait/                   # Wait task normalizer
├── aggregate/              # Aggregate task normalizer
├── composite/              # Composite task normalizer
└── signal/                 # Signal task normalizer
```

### Core Interfaces

```go
// engine/task2/normalizer.go
package task2

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

// NormalizerFactory creates appropriate normalizers
type NormalizerFactory interface {
    CreateNormalizer(taskType task.Type) (TaskNormalizer, error)
}
```

### Factory Implementation

```go
// engine/task2/factory.go
package task2

type DefaultNormalizerFactory struct {
    templateEngine TemplateEngine
    contextBuilder *shared.ContextBuilder
    envMerger      *core.EnvMerger
}

func NewNormalizerFactory(
    engine TemplateEngine,
    builder *shared.ContextBuilder,
    merger *core.EnvMerger,
) NormalizerFactory {
    return &DefaultNormalizerFactory{
        templateEngine: engine,
        contextBuilder: builder,
        envMerger:      merger,
    }
}

func (f *DefaultNormalizerFactory) CreateNormalizer(taskType task.Type) (TaskNormalizer, error) {
    switch taskType {
    case task.TaskTypeBasic:
        return basic.NewNormalizer(f.templateEngine, f.contextBuilder), nil
    case task.TaskTypeParallel:
        return parallel.NewNormalizer(f.templateEngine, f.contextBuilder, f), nil
    case task.TaskTypeCollection:
        return collection.NewNormalizer(
            f.templateEngine,
            f.contextBuilder,
            collection.NewConfigBuilder(),
            collection.NewTypeConverter(),
            collection.NewFilterEvaluator(),
        ), nil
    case task.TaskTypeRouter:
        return router.NewNormalizer(f.templateEngine, f.contextBuilder), nil
    case task.TaskTypeWait:
        return wait.NewNormalizer(f.templateEngine, f.contextBuilder), nil
    case task.TaskTypeAggregate:
        return aggregate.NewNormalizer(f.templateEngine, f.contextBuilder), nil
    case task.TaskTypeComposite:
        return composite.NewNormalizer(f.templateEngine, f.contextBuilder), nil
    case task.TaskTypeSignal:
        return signal.NewNormalizer(f.templateEngine, f.contextBuilder), nil
    default:
        return nil, fmt.Errorf("unsupported task type: %s", taskType)
    }
}

// Component normalizers from core package
func (f *DefaultNormalizerFactory) CreateAgentNormalizer() *core.AgentNormalizer {
    return core.NewAgentNormalizer(f.templateEngine, f.envMerger)
}

func (f *DefaultNormalizerFactory) CreateToolNormalizer() *core.ToolNormalizer {
    return core.NewToolNormalizer(f.templateEngine, f.envMerger)
}

// Transition normalizers from core package
func (f *DefaultNormalizerFactory) CreateSuccessTransitionNormalizer() *core.SuccessTransitionNormalizer {
    return core.NewSuccessTransitionNormalizer(f.templateEngine)
}

func (f *DefaultNormalizerFactory) CreateErrorTransitionNormalizer() *core.ErrorTransitionNormalizer {
    return core.NewErrorTransitionNormalizer(f.templateEngine)
}
```

### Task-Specific Normalizer Example

```go
// engine/task2/parallel/normalizer.go
package parallel

type Normalizer struct {
    templateEngine    TemplateEngine
    contextBuilder    *shared.ContextBuilder
    normalizerFactory task2.NormalizerFactory
}

func NewNormalizer(
    engine TemplateEngine,
    builder *shared.ContextBuilder,
    factory task2.NormalizerFactory,
) *Normalizer {
    return &Normalizer{
        templateEngine:    engine,
        contextBuilder:    builder,
        normalizerFactory: factory,
    }
}

func (n *Normalizer) Normalize(config *task.Config, ctx *shared.NormalizationContext) error {
    // Validate parallel task
    if config.Type != task.TaskTypeParallel {
        return fmt.Errorf("expected parallel task, got %s", config.Type)
    }

    // Normalize base fields
    if err := n.normalizeBaseFields(config, ctx); err != nil {
        return fmt.Errorf("failed to normalize base fields: %w", err)
    }

    // Normalize sub-tasks recursively
    for i, subTask := range config.Tasks {
        subNormalizer, err := n.normalizerFactory.CreateNormalizer(subTask.Type)
        if err != nil {
            return fmt.Errorf("failed to create normalizer for sub-task %d: %w", i, err)
        }

        subCtx := n.buildSubTaskContext(ctx, config, &subTask)
        if err := subNormalizer.Normalize(&subTask, subCtx); err != nil {
            return fmt.Errorf("failed to normalize sub-task %s: %w", subTask.ID, err)
        }

        config.Tasks[i] = subTask
    }

    return nil
}

func (n *Normalizer) Type() task.Type {
    return task.TaskTypeParallel
}
```

### Core Services Implementations

```go
// engine/task2/core/config_normalizer.go
package core

// ConfigNormalizer orchestrates the entire normalization process
type ConfigNormalizer struct {
    factory        NormalizerFactory
    envMerger      *core.EnvMerger
    contextBuilder *shared.ContextBuilder
}

func (cn *ConfigNormalizer) NormalizeTask(
    workflowState *workflow.State,
    workflowConfig *workflow.Config,
    taskConfig *task.Config,
) error {
    // Environment merging logic
    // Context building
    // Delegate to appropriate task normalizer via factory
}

// engine/task2/core/output_transformer.go
package core

// OutputTransformer handles output normalization
type OutputTransformer struct {
    templateEngine TemplateEngine
}

func (ot *OutputTransformer) TransformOutput(
    output *core.Output,
    outputsConfig *core.Input,
    ctx *shared.NormalizationContext,
) (*core.Output, error) {
    // Output transformation logic from ConfigNormalizer
}

// engine/task2/core/agent_normalizer.go
package core

// AgentNormalizer handles agent component normalization
type AgentNormalizer struct {
    templateEngine TemplateEngine
    envMerger      *EnvMerger
}

func (n *AgentNormalizer) NormalizeAgent(
    agentConfig *agent.Config,
    ctx *shared.NormalizationContext,
    parentAction string,
) error {
    // Merge environments
    // Process templates for agent fields
    // Handle action inheritance
}

// engine/task2/core/tool_normalizer.go
package core

// ToolNormalizer handles tool component normalization
type ToolNormalizer struct {
    templateEngine TemplateEngine
    envMerger      *EnvMerger
}

func (n *ToolNormalizer) NormalizeTool(
    toolConfig *tool.Config,
    ctx *shared.NormalizationContext,
) error {
    // Merge environments
    // Process templates for tool fields
}

// engine/task2/core/success_transition.go
package core

// SuccessTransitionNormalizer handles success transition normalization
type SuccessTransitionNormalizer struct {
    templateEngine TemplateEngine
}

func (n *SuccessTransitionNormalizer) Normalize(
    transition *core.SuccessTransition,
    ctx *shared.NormalizationContext,
) error {
    // Process templates in transition fields
}

// engine/task2/core/error_transition.go
package core

// ErrorTransitionNormalizer handles error transition normalization
type ErrorTransitionNormalizer struct {
    templateEngine TemplateEngine
}

func (n *ErrorTransitionNormalizer) Normalize(
    transition *core.ErrorTransition,
    ctx *shared.NormalizationContext,
) error {
    // Process templates in transition fields
}
```

## Implementation Phases

### Phase 1: Foundation (Week 1)

1. **Create Core Infrastructure**

    - Define interfaces in `engine/task2/normalizer.go`
    - Move shared components to `engine/task2/shared/`
    - Implement factory pattern
    - Create orchestrator

2. **Testing Infrastructure**
    - Set up test framework for new architecture
    - Create mock implementations
    - Port relevant tests from existing package

### Phase 2: Basic Normalizers (Week 2)

1. **Basic Task Normalizer**

    - Simplest implementation
    - Template processing for ID, action, with, env
    - Comprehensive tests

2. **Parallel Task Normalizer**
    - Recursive normalization using factory
    - Sub-task context building
    - Integration tests

### Phase 3: Complex Normalizers (Week 3)

1. **Collection Task Normalizer**

    - Port existing CollectionNormalizer
    - Refactor to new interface
    - Move utilities to collection package

2. **Wait/Router/Signal Normalizers**

    - Signal context handling
    - Condition field normalization
    - Processor field handling

3. **Component and Transition Normalizers**
    - Agent normalizer in core package
    - Tool normalizer in core package
    - Success/Error transition normalizers in core package

### Phase 4: Integration and Migration (Week 4)

1. **Factory Implementation**

    - Complete factory with all normalizers
    - Component normalizer creation methods
    - Transition normalizer creation methods

2. **Migration Layer**

    - Adapter from old to new API
    - Feature flag for gradual rollout
    - Performance benchmarks

3. **Testing and Documentation**
    - Integration tests
    - Performance validation
    - Complete documentation

## Migration Strategy

### Backward Compatibility

Since Compozy is in active development (per project rules), we have flexibility:

- No backward compatibility required
- Can make breaking changes
- Focus on best architecture

### Migration Steps

1. **Parallel Development**

    - Keep old normalizer running
    - Develop new architecture alongside
    - No immediate integration

2. **Integration Testing**

    - Create comprehensive test suite
    - Compare outputs between old/new
    - Benchmark performance

3. **Gradual Migration**

    - Feature flag: `USE_NEW_NORMALIZER`
    - Run both in parallel initially
    - Monitor for discrepancies

4. **Cutover**
    - Switch to new normalizer
    - Remove old implementation
    - Update all imports

## Nested Parent Context Solution

### Problem Statement

The current normalization system only provides immediate parent access in templates via `.parent`. This prevents accessing grandparents or higher ancestors (e.g., `.parent.parent.input`), which is essential for deeply nested task hierarchies:

- collection → composite → parallel → basic

### Solution: Recursive Parent Context with Ristretto Caching

Implement a recursive map building pattern with Ristretto caching for performance:

```go
// engine/task2/shared/context.go - Enhanced ContextBuilder

import (
    "github.com/dgraph-io/ristretto"
)

type ContextBuilder struct {
    // Ristretto cache for parent contexts
    parentContextCache *ristretto.Cache[*task.Config, map[string]any]
}

func NewContextBuilder() (*ContextBuilder, error) {
    cache, err := ristretto.NewCache(&ristretto.Config[*task.Config, map[string]any]{
        NumCounters: 1000,  // 10x expected unique parent contexts
        MaxCost:     100,   // Max 100 parent contexts cached
        BufferItems: 64,    // Recommended buffer size
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create parent context cache: %w", err)
    }

    return &ContextBuilder{
        parentContextCache: cache,
    }, nil
}

// BuildParentContext recursively builds parent context chain
func (cb *ContextBuilder) BuildParentContext(
    ctx *NormalizationContext,
    taskConfig *task.Config,
    depth int,
) map[string]any {
    const maxParentDepth = 10 // Prevent infinite recursion

    if taskConfig == nil || depth >= maxParentDepth {
        return nil
    }

    // Check cache first
    if cached, found := cb.parentContextCache.Get(taskConfig); found {
        return cached
    }

    // Build parent context map
    parentMap := map[string]any{
        "id":     taskConfig.ID,
        "type":   taskConfig.Type,
        "action": taskConfig.Action,
        "with":   taskConfig.With,
        "env":    taskConfig.Env,
    }

    // Add runtime state if available
    if ctx.WorkflowState != nil && ctx.WorkflowState.Tasks != nil {
        if taskState, exists := ctx.WorkflowState.Tasks[taskConfig.ID]; exists {
            parentMap["input"] = taskState.Input
            parentMap["output"] = taskState.Output
            parentMap["status"] = taskState.Status
            if taskState.Error != nil {
                parentMap["error"] = taskState.Error
            }
        }
    }

    // Find and recursively add grandparent
    grandParentTask := cb.findParentTask(ctx, taskConfig)
    if grandParentTask != nil {
        parentMap["parent"] = cb.BuildParentContext(ctx, grandParentTask, depth+1)
    }

    // Store in cache with cost of 1
    cb.parentContextCache.Set(taskConfig, parentMap, 1)
    cb.parentContextCache.Wait()

    return parentMap
}
```

### Key Features

1. **Ristretto Caching**: High-performance caching prevents redundant parent chain building
2. **Recursive Building**: Each parent map contains its own parent map, enabling `.parent.parent.parent` syntax
3. **Runtime State Integration**: Includes task execution state (input/output/status) when available
4. **Depth Limiting**: Prevents infinite recursion with configurable max depth
5. **Lazy Evaluation**: Parent chains only built when accessed in templates

### Integration Points

1. **Factory Initialization**: Create context builder with cache
2. **Sub-task Context**: Use recursive parent building for all task types
3. **Cache Lifecycle**: Clear cache at workflow start to prevent stale data

### Template Usage

After implementation, templates can access arbitrary parent depth:

```
{{.parent.input}}                    # Immediate parent input
{{.parent.parent.with.someValue}}    # Grandparent's with parameter
{{.parent.parent.parent.output}}     # Great-grandparent's output
```

### Performance Considerations

- Ristretto provides O(1) cache lookups with minimal memory overhead
- Cache size limited to prevent unbounded growth
- Parent chains built once per workflow execution and reused

## Testing Strategy

### Unit Tests

Each normalizer will have comprehensive unit tests:

```go
// engine/task2/basic/normalizer_test.go
func TestBasicNormalizer(t *testing.T) {
    t.Run("Should normalize ID with templates", func(t *testing.T) {
        // Test template processing
    })

    t.Run("Should handle missing fields", func(t *testing.T) {
        // Test error cases
    })

    t.Run("Should preserve non-template values", func(t *testing.T) {
        // Test passthrough
    })
}
```

### Integration Tests

Test the complete normalization flow:

```go
// engine/task2/integration_test.go
func TestNormalizationFlow(t *testing.T) {
    t.Run("Should normalize complex workflow", func(t *testing.T) {
        // Test with real workflow configs
    })
}
```

## Success Criteria

1. **Architecture Quality**

    - Full SOLID compliance
    - Clear separation of concerns
    - Easy to extend with new task types

2. **Functionality**

    - All existing features preserved
    - No regression in normalization logic
    - Improved error messages

3. **Performance**

    - No degradation in performance
    - Reduced memory allocations
    - Better CPU utilization

4. **Maintainability**
    - Reduced cyclomatic complexity
    - Improved test coverage (>80%)
    - Clear documentation

## Risk Mitigation

1. **Complexity Risk**

    - Start with simplest task types
    - Incremental migration
    - Comprehensive testing

2. **Integration Risk**

    - Parallel running with comparison
    - Feature flags for rollback
    - Extensive integration tests

3. **Performance Risk**
    - Benchmark at each phase
    - Profile memory usage
    - Optimize hot paths

## Future Enhancements

1. **Plugin Architecture**

    - Dynamic task type registration
    - External normalizer plugins
    - Runtime extensibility

2. **Caching Layer**

    - Cache normalized configs
    - Template compilation cache
    - Context reuse optimization

3. **Validation Framework**
    - Pre-normalization validation
    - Post-normalization verification
    - Schema-based validation
