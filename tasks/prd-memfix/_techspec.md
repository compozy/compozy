# Technical Specification: Memory Task Integration Fix

## Overview

This technical specification provides detailed implementation guidance for fixing the memory task integration issue in Compozy. The fix involves completing the Task2 factory implementation to support memory task normalizer and response handler components.

## Architecture Context

### Current Memory System Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Workflow      │    │   Task Router   │    │  Memory Task    │
│   Executor      │───▶│   (worker)      │───▶│   Executor      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                      │
                                                      ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Temporal      │    │   ExecuteMemory │    │   Task2         │
│   Activity      │◀───│   Activity      │───▶│   Factory       │ ❌ FAILS HERE
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                                             │
         ▼                                             ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Memory        │    │   Memory        │    │   Memory        │
│   Operations    │    │   Service       │    │   Manager       │
│   Use Case      │    │   Layer         │    │   (Redis)       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Current Task2 Factory Pattern

All task types follow this pattern:

1. **Normalizer**: Processes task configuration and applies templates
2. **Response Handler**: Processes task execution results and formats output
3. **Factory Integration**: Both components registered in factory switch statements

## Problem Analysis

### Root Cause Details

**File**: `engine/task2/factory.go`

```go
// ❌ MISSING: Memory task case in CreateNormalizer
func (f *DefaultNormalizerFactory) CreateNormalizer(taskType task.Type) (contracts.TaskNormalizer, error) {
    switch taskType {
    case task.TaskTypeBasic, "":
        return basic.NewNormalizer(f.templateEngine), nil
    case task.TaskTypeParallel:
        return parallel.NewNormalizer(f.templateEngine, f.contextBuilder, f), nil
    // ... other task types ...
    case task.TaskTypeSignal:
        return signal.NewNormalizer(f.templateEngine, f.contextBuilder), nil
    // ❌ MISSING: case task.TaskTypeMemory:
    default:
        return nil, fmt.Errorf("unsupported task type: %s", taskType)
    }
}

// ❌ MISSING: Memory task case in CreateResponseHandler
func (f *DefaultNormalizerFactory) CreateResponseHandler(taskType task.Type) (shared.TaskResponseHandler, error) {
    // ... existing switch statement ...
    case task.TaskTypeAggregate:
        return aggregate.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
    // ❌ MISSING: case task.TaskTypeMemory:
    default:
        return nil, fmt.Errorf("unsupported task type for response handler: %s", taskType)
    }
}
```

### Failure Flow Analysis

1. **Memory Task Execution** (`exec_memory.go:69`)

    ```go
    normalizer, err := a.task2Factory.CreateNormalizer(task.TaskTypeMemory)
    ```

    - Calls factory with `task.TaskTypeMemory`
    - Factory hits `default` case
    - Returns `"unsupported task type: memory"` error
    - Execution fails with: `"failed to create memory normalizer: unsupported task type: memory"`

2. **Response Handler Creation** (`exec_memory.go:104`)

    ```go
    handler, err := a.task2Factory.CreateResponseHandler(task.TaskTypeMemory)
    ```

    - Same pattern - hits default case
    - Returns `"unsupported task type for response handler: memory"` error
    - Execution fails with: `"failed to create memory response handler: unsupported task type for response handler: memory"`

## Solution Implementation

### Approach 1: Immediate Fix (Recommended)

**Objective**: Minimal changes to unblock memory task functionality immediately.

**Implementation**:

1. **Modify factory.go CreateNormalizer method**:

```go
// Add this case around line 81, after TaskTypeSignal
case task.TaskTypeMemory:
    return basic.NewNormalizer(f.templateEngine), nil
```

2. **Modify factory.go CreateResponseHandler method**:

```go
// Add this case around line 151, after TaskTypeAggregate
case task.TaskTypeMemory:
    return basic.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
```

**Rationale**:

- Memory tasks have simple normalization needs similar to basic tasks
- Basic response handler provides standard output processing
- Minimal risk, immediate functionality restoration
- Follows established dependency patterns

### Approach 2: Dedicated Implementation (Future Enhancement)

**Objective**: Create memory-specific normalizer and response handler for optimal integration.

**Directory Structure**:

```
engine/task2/memory/
├── normalizer.go
├── normalizer_test.go
├── response_handler.go
└── response_handler_test.go
```

**Memory Normalizer Implementation**:

```go
// engine/task2/memory/normalizer.go
package memory

import (
    "github.com/compozy/compozy/engine/task"
    "github.com/compozy/compozy/engine/task2/contracts"
    "github.com/compozy/compozy/engine/task2/shared"
    "github.com/compozy/compozy/pkg/tplengine"
)

type Normalizer struct {
    *shared.BaseNormalizer
}

func NewNormalizer(templateEngine *tplengine.TemplateEngine, contextBuilder *shared.ContextBuilder) *Normalizer {
    return &Normalizer{
        BaseNormalizer: shared.NewBaseNormalizer(
            templateEngine,
            contextBuilder,
            task.TaskTypeMemory,
            nil, // Use default filter
        ),
    }
}

func (n *Normalizer) Normalize(config *task.Config, ctx contracts.NormalizationContext) error {
    if config == nil {
        return nil
    }

    // Apply base normalization
    if err := n.BaseNormalizer.Normalize(config, ctx); err != nil {
        return err
    }

    // Memory-specific validation
    if config.MemoryRef == "" {
        return fmt.Errorf("memory_ref is required for memory tasks")
    }

    if config.KeyTemplate == "" {
        return fmt.Errorf("key_template is required for memory tasks")
    }

    // Validate operation type
    if !isValidMemoryOperation(config.Operation) {
        return fmt.Errorf("invalid memory operation: %s", config.Operation)
    }

    return nil
}

func isValidMemoryOperation(op task.MemoryOpType) bool {
    validOps := []task.MemoryOpType{
        task.MemoryOpRead, task.MemoryOpWrite, task.MemoryOpAppend,
        task.MemoryOpDelete, task.MemoryOpFlush, task.MemoryOpHealth,
        task.MemoryOpClear, task.MemoryOpStats,
    }

    for _, validOp := range validOps {
        if op == validOp {
            return true
        }
    }
    return false
}
```

**Memory Response Handler Implementation**:

```go
// engine/task2/memory/response_handler.go
package memory

import (
    "context"

    "github.com/compozy/compozy/engine/task"
    "github.com/compozy/compozy/engine/task2/shared"
    "github.com/compozy/compozy/pkg/tplengine"
)

type ResponseHandler struct {
    baseHandler    *shared.BaseResponseHandler
    templateEngine *tplengine.TemplateEngine
    contextBuilder *shared.ContextBuilder
}

func NewResponseHandler(
    templateEngine *tplengine.TemplateEngine,
    contextBuilder *shared.ContextBuilder,
    baseHandler *shared.BaseResponseHandler,
) *ResponseHandler {
    if baseHandler == nil {
        panic("baseHandler cannot be nil")
    }
    return &ResponseHandler{
        baseHandler:    baseHandler,
        templateEngine: templateEngine,
        contextBuilder: contextBuilder,
    }
}

func (h *ResponseHandler) HandleResponse(
    ctx context.Context,
    input *shared.ResponseInput,
) (*shared.ResponseOutput, error) {
    // Validate input
    if err := h.baseHandler.ValidateInput(input); err != nil {
        return nil, err
    }

    // Validate task type
    if input.TaskConfig.Type != task.TaskTypeMemory {
        return nil, &shared.ValidationError{
            Field:   "task_type",
            Message: "handler type does not match task type",
        }
    }

    // Memory-specific response processing
    if input.TaskState.Output != nil {
        // Add memory operation metadata
        if output, ok := input.TaskState.Output.AsMap(); ok {
            output["operation"] = input.TaskConfig.Operation
            output["memory_ref"] = input.TaskConfig.MemoryRef

            // Update output with enhanced metadata
            input.TaskState.Output = &task.Output{}
            for k, v := range output {
                (*input.TaskState.Output)[k] = v
            }
        }
    }

    // Delegate to base handler for standard processing
    return h.baseHandler.ProcessMainTaskResponse(ctx, input)
}

func (h *ResponseHandler) Type() task.Type {
    return task.TaskTypeMemory
}
```

**Factory Integration**:

```go
// Update factory.go with dedicated implementations
case task.TaskTypeMemory:
    return memory.NewNormalizer(f.templateEngine, f.contextBuilder), nil

case task.TaskTypeMemory:
    return memory.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
```

## Implementation Details

### File Changes Required

#### Immediate Fix (Approach 1)

**File**: `engine/task2/factory.go`

**Change 1** - Line ~81 in `CreateNormalizer`:

```diff
     case task.TaskTypeSignal:
         return signal.NewNormalizer(f.templateEngine, f.contextBuilder), nil
+    case task.TaskTypeMemory:
+        return basic.NewNormalizer(f.templateEngine), nil
     default:
         return nil, fmt.Errorf("unsupported task type: %s", taskType)
```

**Change 2** - Line ~151 in `CreateResponseHandler`:

```diff
     case task.TaskTypeAggregate:
         return aggregate.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
+    case task.TaskTypeMemory:
+        return basic.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
     default:
         return nil, fmt.Errorf("unsupported task type for response handler: %s", taskType)
```

### Testing Strategy

#### Unit Tests

**File**: `engine/task2/factory_test.go`

```go
func TestCreateNormalizer_Memory(t *testing.T) {
    factory := createTestFactory(t)

    normalizer, err := factory.CreateNormalizer(task.TaskTypeMemory)

    assert.NoError(t, err)
    assert.NotNil(t, normalizer)
    assert.IsType(t, &basic.Normalizer{}, normalizer)
}

func TestCreateResponseHandler_Memory(t *testing.T) {
    factory := createTestFactory(t)

    handler, err := factory.CreateResponseHandler(task.TaskTypeMemory)

    assert.NoError(t, err)
    assert.NotNil(t, handler)
    assert.Equal(t, task.TaskTypeMemory, handler.Type())
}
```

#### Integration Tests

**File**: `engine/task/activities/exec_memory_test.go`

```go
func TestExecuteMemory_FullFlow(t *testing.T) {
    // Setup test dependencies
    templateEngine := tplengine.New()
    factory, err := task2.NewFactory(&task2.FactoryConfig{
        TemplateEngine: templateEngine,
        EnvMerger:      core.NewEnvMerger(),
    })
    require.NoError(t, err)

    // Create ExecuteMemory activity
    activity := &ExecuteMemory{
        task2Factory: factory,
        // ... other dependencies
    }

    // Test input
    input := &ExecuteMemoryInput{
        TaskConfig: &task.Config{
            Type:        task.TaskTypeMemory,
            Operation:   task.MemoryOpWrite,
            MemoryRef:   "test_memory",
            KeyTemplate: "test_key",
            Payload:     map[string]any{"message": "test"},
        },
        // ... other fields
    }

    // Execute
    response, err := activity.Run(context.Background(), input)

    // Verify
    assert.NoError(t, err)
    assert.NotNil(t, response)
    assert.Equal(t, task.StatusCompleted, response.Status)
}
```

### Validation & Quality Assurance

#### Pre-commit Checks

```bash
# Run linting
make lint

# Run tests
make test

# Run specific memory task tests
go test -v ./engine/task/activities -run TestExecuteMemory
go test -v ./engine/task2 -run TestCreateNormalizer_Memory
```

#### Manual Testing

**Test Workflow**:

```yaml
# test-memory-workflow.yaml
name: test_memory_workflow
version: 0.1.0
description: Test memory task integration

tasks:
    - id: test_memory_write
      type: memory
      operation: write
      memory_ref: test_memory
      key_template: "test:{{.workflow.id}}"
      payload:
          message: "Hello, Memory!"
          timestamp: "{{now}}"

    - id: test_memory_read
      type: memory
      operation: read
      memory_ref: test_memory
      key_template: "test:{{.workflow.id}}"
```

**Expected Results**:

- Write operation creates memory entry
- Read operation retrieves stored data
- No "unsupported task type" errors

### Performance Considerations

#### Memory Impact

- **Normalizer**: Minimal overhead, reuses basic implementation
- **Response Handler**: Standard processing with metadata enhancement
- **Factory Calls**: O(1) switch statement lookup, no performance impact

#### Scalability

- Memory operations scale with Redis cluster capacity
- Template resolution scales linearly with complexity
- No additional bottlenecks introduced by factory changes

### Security Considerations

#### Current Security Posture

- Template injection risks exist in key templates (identified in analysis)
- Redis key sanitization needed
- Rate limiting gaps for memory operations

#### Immediate Fix Security Impact

- **Low Risk**: Using basic normalizer/handler components
- **No New Vulnerabilities**: Reuses existing, tested implementations
- **Security Hardening**: Should be addressed in Phase 2 (separate from this fix)

### Monitoring & Observability

#### Metrics to Track

- Memory task execution success/failure rates
- Memory operation latency by type
- Template resolution performance
- Redis operation metrics

#### Logging Enhancements

```go
// Add to exec_memory.go
log.Info("Memory task execution started",
    "operation", input.TaskConfig.Operation,
    "memory_ref", input.TaskConfig.MemoryRef,
    "workflow_id", input.WorkflowID)
```

## Rollout Plan

### Phase 1: Implementation (Day 1)

1. **Code Changes** (2 hours)
    - Modify factory.go with memory task cases
    - Add basic unit tests
2. **Local Testing** (2 hours)
    - Verify factory methods work
    - Test simple memory workflow locally
3. **Code Review** (2 hours)
    - Submit PR with changes
    - Address review feedback

### Phase 2: Validation (Day 2)

1. **Integration Testing** (4 hours)
    - Run full test suite
    - Test all memory operations
    - Verify error handling
2. **Performance Testing** (2 hours)
    - Measure baseline performance
    - Validate no regressions
3. **Documentation** (2 hours)
    - Update memory task usage docs
    - Add troubleshooting guide

### Phase 3: Deployment (Day 3)

1. **Staging Deployment** (2 hours)
    - Deploy to staging environment
    - Run acceptance tests
2. **Production Deployment** (2 hours)
    - Deploy to production
    - Monitor metrics and logs
3. **Validation** (4 hours)
    - Verify production functionality
    - Monitor for errors
    - Document any issues

## Risk Mitigation

### Technical Risks

1. **ResourceRegistry Configuration**
    - **Risk**: Memory manager fails to initialize
    - **Detection**: Check worker startup logs for memory manager warnings
    - **Mitigation**: Ensure autoload configuration includes memory resources

2. **Basic Implementation Limitations**
    - **Risk**: Memory-specific features may not work optimally
    - **Detection**: Monitor memory operation success rates
    - **Mitigation**: Plan dedicated implementation in future phases

### Rollback Plan

If issues arise, rollback is simple:

1. **Remove factory cases**: Comment out the two added case statements
2. **Redeploy**: Memory tasks will fail with original error (expected)
3. **Investigate**: Debug issues in non-production environment

### Success Criteria

#### Immediate (Day 1)

- [ ] Factory methods return normalizer/handler for memory tasks
- [ ] No "unsupported task type" errors in local testing
- [ ] Unit tests pass

#### Short-term (Day 3)

- [ ] Memory workflows execute successfully in production
- [ ] All memory operations (read, write, append, delete, flush, health, clear, stats) work
- [ ] No performance regressions detected
- [ ] Error rates within acceptable thresholds

#### Long-term (Week 1)

- [ ] Memory task usage increases (indicating feature adoption)
- [ ] No memory-related production incidents
- [ ] Positive user feedback on memory functionality

## Future Enhancements

### Dedicated Memory Components (Phase 2)

- Implement memory-specific normalizer with enhanced validation
- Create memory-specific response handler with operation metadata
- Add memory operation performance optimizations

### Security Hardening (Phase 2)

- Template injection validation for key templates
- Redis key sanitization to prevent information leakage
- Rate limiting for memory operations

### Advanced Features (Phase 3)

- Memory operation batching for high-throughput scenarios
- Cross-memory-instance operations
- Advanced memory analytics and insights

## Conclusion

This technical specification provides a clear, low-risk path to fix the memory task integration issue. The immediate fix using basic normalizer/handler components will restore functionality while preserving the option for future enhancements. The implementation is straightforward, well-tested, and follows established project patterns.
