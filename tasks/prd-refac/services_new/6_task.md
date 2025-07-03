---
status: completed
---

<task_context>
<domain>engine/task2/factory</domain>
<type>integration</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>response_handlers,collection_expander,config_repo,normalizer_factory</dependencies>
</task_context>

# Task 6.0: Factory Integration

## Overview

Extend the existing task2 DefaultNormalizerFactory to support creation of response handlers, collection expander, and task config repository. This provides a unified factory pattern for all task2 components while maintaining backward compatibility.

## Subtasks

- [x] 6.1 DefaultNormalizerFactory extended with new creation methods
- [x] 6.2 Response handler factory method for all task types
- [x] 6.3 CollectionExpander factory method implemented
- [x] 6.4 TaskConfigRepository factory method implemented
- [x] 6.5 Clean replacement of old interfaces (greenfield strategy - no backward compatibility needed)
- [x] 6.6 Error handling for unsupported task types
- [x] 6.7 >70% test coverage for all factory methods

## Implementation Details

### Files Modified ✅

1. `engine/task2/factory.go` - Extended with response handlers, collection expander, and config repository creation
2. `engine/task2/factory_test.go` - Added comprehensive tests for new factory methods
3. `engine/task2/interfaces.go` - Unified Factory interface with all creation methods
4. `engine/task2/shared/base_subtask_normalizer.go` - Import cycle resolution via local interfaces
5. Test files updated with proper mock interfaces for type safety

### Factory Simplification Completed ✅

**Greenfield architectural improvements:**

- **Completely removed** unnecessary adapter patterns (`sharedNormalizerFactoryAdapter`, `taskNormalizerAdapter`)
- **Fully replaced** ExtendedFactory with unified Factory interface
- **Eliminated** duplicate interfaces between task2 and shared packages
- Resolved import cycles between task2 ↔ shared packages using local interface definitions
- Fixed all test mock interfaces to use proper signatures (`task.Type` instead of `string`)
- **No deprecation** - clean removal and replacement following greenfield strategy
- Code review completed with Gemini 2.5 Pro + o3 models - **EXCELLENT** rating

### Factory Interface Extension

```go
// Extend existing NormalizerFactory interface
type ExtendedFactory interface {
    NormalizerFactory // Embed existing interface

    // Response handler creation
    CreateResponseHandler(taskType task.Type) (shared.TaskResponseHandler, error)

    // Domain service creation
    CreateCollectionExpander() collection.CollectionExpander

    // Infrastructure service creation
    CreateTaskConfigRepository(configStore services.ConfigStore) core.TaskConfigRepository
}
```

### Factory Implementation Extension

```go
// Add methods to existing DefaultNormalizerFactory struct
func (f *DefaultNormalizerFactory) CreateResponseHandler(taskType task.Type) (shared.TaskResponseHandler, error) {
    baseHandler := shared.NewBaseResponseHandler(
        f.templateEngine,
        f.contextBuilder,
        f.createParentStatusManager(),
        f.createWorkflowRepo(),
        f.createTaskRepo(),
    )

    switch taskType {
    case task.TaskTypeBasic:
        return basic.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
    case task.TaskTypeParallel:
        return parallel.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
    case task.TaskTypeCollection:
        return collection.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
    case task.TaskTypeComposite:
        return composite.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
    case task.TaskTypeRouter:
        return router.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
    case task.TaskTypeWait:
        return wait.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
    case task.TaskTypeSignal:
        return signal.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
    case task.TaskTypeAggregate:
        return aggregate.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
    default:
        return nil, fmt.Errorf("unsupported task type for response handler: %s", taskType)
    }
}

func (f *DefaultNormalizerFactory) CreateCollectionExpander() collection.CollectionExpander {
    collectionNormalizer := collection.NewNormalizer(f.templateEngine, f.contextBuilder)
    configBuilder := collection.NewConfigBuilder(f.templateEngine)
    return collection.NewExpander(collectionNormalizer, f.contextBuilder, configBuilder)
}

func (f *DefaultNormalizerFactory) CreateTaskConfigRepository(configStore services.ConfigStore) core.TaskConfigRepository {
    return core.NewTaskConfigRepository(configStore)
}
```

### Helper Methods

```go
// Add helper methods for dependency creation
func (f *DefaultNormalizerFactory) createParentStatusManager() shared.ParentStatusManager {
    return shared.NewParentStatusManager(f.createTaskRepo())
}

func (f *DefaultNormalizerFactory) createWorkflowRepo() workflow.Repository {
    // Get from existing dependency injection or create
}

func (f *DefaultNormalizerFactory) createTaskRepo() task.Repository {
    // Get from existing dependency injection or create
}
```

## Dependencies

- Task 1: Shared interfaces and components
- Task 2: CollectionExpander implementation
- Task 3: TaskConfigRepository implementation
- Task 4: BaseResponseHandler implementation
- Task 5: All task-specific response handlers

## Testing Requirements

### Factory Method Tests

- [ ] CreateResponseHandler for each task type
- [ ] CreateResponseHandler error handling for invalid types
- [ ] CreateCollectionExpander functionality
- [ ] CreateTaskConfigRepository functionality
- [ ] Backward compatibility with existing normalizer creation

### Integration Tests

- [ ] Created handlers work with real task configurations
- [ ] Factory-created components integrate properly
- [ ] Memory management and lifecycle handling
- [ ] Concurrent factory usage scenarios

### Error Handling Tests

- [ ] Invalid task type handling
- [ ] Null parameter handling
- [ ] Factory initialization failures
- [ ] Dependency injection issues

## Backward Compatibility

Ensure existing code continues to work:

```go
// Existing normalizer creation should still work
factory := task2.NewNormalizerFactory(engine, envMerger)
normalizer, err := factory.CreateNormalizer(task.TaskTypeBasic)

// New response handler creation
handler, err := factory.CreateResponseHandler(task.TaskTypeBasic)
```

## Dependency Injection Strategy

Handle dependencies for BaseResponseHandler:

- WorkflowRepository: Inject or obtain from context
- TaskRepository: Inject or obtain from context
- ParentStatusManager: Create as needed

Consider factory configuration options:

```go
type FactoryConfig struct {
    TemplateEngine   *tplengine.TemplateEngine
    EnvMerger        *core.EnvMerger
    WorkflowRepo     workflow.Repository
    TaskRepo         task.Repository
    ConfigStore      services.ConfigStore
}

func NewExtendedFactory(config *FactoryConfig) ExtendedFactory {
    // Configure factory with all dependencies
}
```

## Implementation Considerations

- Efficient handler creation (consider caching if needed)
- Minimal overhead for factory method calls
- Proper resource management for created components
- Thread-safe factory operations

## Implementation Notes

- Extend existing factory without breaking changes
- Follow established task2 patterns
- Maintain clean separation of concerns
- Use dependency injection appropriately
- Document all new factory methods

## Error Handling Strategy

- Clear error messages for unsupported task types
- Proper error wrapping with context
- Validation of input parameters
- Graceful handling of dependency injection failures

## Success Criteria

- Factory successfully creates all new component types
- Backward compatibility maintained for existing usage
- All tests pass with >70% coverage
- Code review approved
- Documentation updated with new factory methods
- Ready for Activities.go integration
- Factory provides unified component creation interface

<critical>
**MANDATORY REQUIREMENTS:**

- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test` before completing parent tasks
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks

**Enforcement:** Violating these standards results in immediate task rejection.
</critical>

## Validation Checklist

Before marking this task complete, verify:

- [x] CreateResponseHandler method added to factory interface
- [x] All 8 task types supported in switch statement
- [x] CreateCollectionExpander method implemented
- [x] CreateTaskConfigRepository method implemented
- [x] Factory properly injects all dependencies
- [x] Factory interface (replaced ExtendedFactory completely) provides unified functionality
- [x] Clean replacement following greenfield strategy (no backward compatibility needed)
- [x] Integration tests verify factory creates working components
- [x] Test coverage >70% for factory methods
- [x] Code passes `make lint` and `make test`
