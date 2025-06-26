# Task2 Integration Plan

## Overview

Direct integration of the completed task2 package to replace the legacy `pkg/normalizer` in the existing Compozy system. The refactored task2 package is complete and tested - now we need to integrate it.

## Current State

### ✅ Completed

- Task2 package fully implemented with modular normalizers
- All task types supported (basic, parallel, collection, router, wait, etc.)
- Comprehensive test coverage
- Code quality standards met

### ❌ Integration Gap

- Current system still uses `pkg/normalizer/config.go`
- Task2 package not connected to the execution flow
- Need simple integration without over-engineering

## Integration Strategy

### Direct Replacement Approach

Replace the old normalizer with task2 orchestrator in minimal steps, avoiding complex migration layers or feature flags.

## Implementation Steps

### Step 1: Replace Use Case Layer (Day 1)

**File: `engine/task/uc/norm_config.go`**

**Changes:**

1. Replace import: `pkg/normalizer` → `engine/task2`
2. Replace struct field: `normalizer *normalizer.ConfigNormalizer` → `orchestrator *task2.ConfigOrchestrator`
3. Update constructor to initialize task2 factory and orchestrator
4. Update method calls to use orchestrator API

**Before:**

```go
import "github.com/compozy/compozy/pkg/normalizer"

type NormalizeConfig struct {
    normalizer *normalizer.ConfigNormalizer
}

func NewNormalizeConfig() *NormalizeConfig {
    return &NormalizeConfig{
        normalizer: normalizer.NewConfigNormalizer(),
    }
}
```

**After:**

```go
import "github.com/compozy/compozy/engine/task2"

type NormalizeConfig struct {
    orchestrator *task2.ConfigOrchestrator
}

func NewNormalizeConfig() (*NormalizeConfig, error) {
    factory, err := task2.NewDefaultNormalizerFactory()
    if err != nil {
        return nil, err
    }

    orchestrator, err := task2.NewConfigOrchestrator(factory)
    if err != nil {
        return nil, err
    }

    return &NormalizeConfig{
        orchestrator: orchestrator,
    }, nil
}
```

### Step 2: Update Method Calls (Day 1)

Replace all normalizer method calls with orchestrator equivalents:

**Task normalization:**

```go
// OLD:
err := uc.normalizer.NormalizeTask(workflowState, workflowConfig, taskConfig)

// NEW:
err := uc.orchestrator.NormalizeTask(workflowState, workflowConfig, taskConfig)
```

**Agent normalization:**

```go
// OLD:
err := uc.normalizer.NormalizeAgentComponent(...)

// NEW:
err := uc.orchestrator.NormalizeAgentComponent(...)
```

**Tool normalization:**

```go
// OLD:
err := uc.normalizer.NormalizeToolComponent(...)

// NEW:
err := uc.orchestrator.NormalizeToolComponent(...)
```

### Step 3: Update Dependency Injection (Day 1)

**Files to update:**

- Any dependency injection setup that creates `NormalizeConfig`
- Update to handle the new constructor signature with error return

### Step 4: Test Integration (Day 2)

1. Run existing test suite
2. Test all task types: basic, parallel, collection, router, wait
3. Test agent and tool normalization
4. Verify no regression in functionality

### Step 5: Clean Up (Day 3)

1. Remove `pkg/normalizer/` package entirely
2. Update any remaining imports across the codebase
3. Clean up unused dependencies

## API Compatibility

### Task2 Orchestrator Methods

The task2 orchestrator provides identical method signatures:

```go
// Both APIs are compatible
NormalizeTask(workflowState, workflowConfig, taskConfig) error
NormalizeAgentComponent(workflowState, workflowConfig, taskConfig, agentConfig, taskConfigs) error
NormalizeToolComponent(workflowState, workflowConfig, taskConfig, toolConfig, taskConfigs) error
```

### Key Differences

1. **Constructor**: Task2 requires factory initialization (returns error)
2. **Dependencies**: Task2 uses internal factory pattern instead of direct dependencies
3. **Implementation**: Task2 uses modular normalizers internally

## Risk Assessment

### Low Risk

- API compatibility maintained
- All functionality preserved in task2
- Comprehensive test coverage exists

### Mitigation

- Keep old package until testing complete
- Test with existing workflows first
- Simple rollback if needed (revert commits)

## Success Criteria

1. ✅ All existing tests pass
2. ✅ No performance regression
3. ✅ All task types work correctly
4. ✅ Agent/tool normalization preserved
5. ✅ Ready for production deployment

## Timeline

- **Day 1**: Replace normalizer in use case layer
- **Day 2**: Test integration and fix issues
- **Day 3**: Clean up and remove old package

**Total: 3 days maximum**

## Files to Modify

### Primary Changes

- `engine/task/uc/norm_config.go` - Main integration point

### Secondary Changes

- Any dependency injection setup
- Import statements across codebase
- Remove `pkg/normalizer/` package

### Testing

- Run existing test suite
- Integration testing with workflows
- Performance validation

## Key Benefits Post-Integration

1. **Maintainability**: Modular normalizers easier to maintain
2. **Extensibility**: New task types easier to add
3. **Code Quality**: SOLID principles compliance
4. **Testing**: Better test isolation and coverage
5. **Architecture**: Clean separation of concerns

## Completion Checklist

- [ ] Replace use case normalizer with orchestrator
- [ ] Update dependency injection
- [ ] Run and fix tests
- [ ] Performance validation
- [ ] Remove old normalizer package
- [ ] Update documentation
- [ ] Deploy to production
