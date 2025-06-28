# Memory Service Implementation Plan

Date: 2025-06-28

## Updated Issue Priority (After Analysis)

### Issues Already Solved ✅

1. **Race Conditions** - Use existing lock manager
2. **AtomicOperations Interface** - Already implemented
3. **ReDoS Vulnerability** - Use existing safe validation
4. **Error Handling** - Use existing typed error system
5. **Instrumentation** - Use existing OpenTelemetry metrics
6. **Security Limits** - Already configurable constants
7. **Rate Limiting** - Already handled at REST API level

### Critical Issues to Fix 🔴

#### 1. Make Append Operation Atomic

**File**: `engine/memory/service/operations.go:172-194`
**Fix**: Wrap in MemoryTransaction like Write operation

```go
func (s *memoryOperationsService) Append(ctx context.Context, req *AppendRequest) (*AppendResponse, error) {
    // Use transaction for atomic append
    tx := NewMemoryTransaction(instance)
    if err := tx.Begin(ctx); err != nil {
        return nil, fmt.Errorf("failed to begin transaction: %w", err)
    }

    // Get current count before append
    beforeCount, _ := instance.Len(ctx)

    // Apply messages
    if err := tx.ApplyMessages(ctx, messages); err != nil {
        if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
            return nil, fmt.Errorf("append failed and rollback failed: %w (original: %w)", rollbackErr, err)
        }
        return nil, fmt.Errorf("append failed, memory restored: %w", err)
    }

    // Commit transaction
    if err := tx.Commit(); err != nil {
        return nil, fmt.Errorf("failed to commit transaction: %w", err)
    }

    // Get new total count
    totalCount, err := instance.Len(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get message count: %w", err)
    }

    return &AppendResponse{
        Success:    true,
        Appended:   totalCount - beforeCount,
        TotalCount: totalCount,
        Key:        resolvedKey,
    }, nil
}
```

#### 2. Fix Clear Operation Backup Claim

**File**: `engine/memory/service/operations.go:263`
**Fix**: Either implement backup or remove the claim

```go
// Option A: Remove false claim
return &ClearResponse{
    Success:         true,
    Key:             resolvedKey,
    MessagesCleared: beforeCount,
    BackupCreated:   false, // Changed from true
}, nil

// Option B: Implement actual backup (if needed)
var backup []llm.Message
if req.Config.Backup {
    backup, _ = instance.Read(ctx) // Store backup somewhere
}
```

#### 3. Add Context Cancellation Checks

**Files**: All operations with loops
**Fix**: Add helper function and use in loops

```go
// Add to service
func checkContextCancellation(ctx context.Context) error {
    select {
    case <-ctx.Done():
        return core.NewMemoryError("CONTEXT_CANCELLED", "operation cancelled", nil)
    default:
        return nil
    }
}

// Use in loops
for i, msg := range messages {
    if err := checkContextCancellation(ctx); err != nil {
        return err
    }
    // ... process message
}
```

### High Priority Fixes 🟡

#### 4. Integrate Lock Manager

**Fix**: Add lock manager to service and use in all operations

```go
type memoryOperationsService struct {
    memoryManager  memcore.ManagerInterface
    templateEngine *tplengine.TemplateEngine
    lockManager    core.LockManager // Add this
}

// In each operation:
lock, err := s.lockManager.AcquireAppendLock(ctx, req.MemoryRef)
if err != nil {
    return nil, core.NewMemoryError(core.ErrCodeLockAcquisition, "failed to acquire lock", nil)
}
defer handleLockRelease(lock, &err)
```

#### 5. Use Existing Validation

**Fix**: Import and use existing validation functions

```go
import valutil "github.com/compozy/compozy/engine/memory/uc"

// Replace current validation with:
if err := valutil.ValidateMemoryRef(req.MemoryRef); err != nil {
    return core.NewMemoryError(core.ErrCodeInvalidInput, err.Error(), nil)
}
```

#### 6. Check AtomicOperations Interface

**Fix**: Add interface check in operations

```go
// In Write/Append operations
if atomicOps, ok := instance.(memcore.AtomicOperations); ok {
    // Use atomic operations
    err = atomicOps.ReplaceMessagesWithMetadata(ctx, key, rawMessages, totalTokens)
} else {
    // Use regular transaction approach
}
```

### Medium Priority Enhancements 🟢

1. **Extend Transaction Support** - Make transaction more flexible
2. **Add Metrics** - Integrate with existing OpenTelemetry
3. **Improve Error Context** - Use typed errors consistently
4. **Add Progress Reporting** - For long operations

## Implementation Steps

### Phase 1: Critical Fixes (Day 1)

1. [ ] Make Append operation atomic using MemoryTransaction
2. [ ] Fix Clear operation backup claim
3. [ ] Add context cancellation checks helper
4. [ ] Run tests to ensure fixes work

### Phase 2: Integration (Day 1-2)

1. [ ] Add lock manager to service constructor
2. [ ] Replace validation with existing safe functions
3. [ ] Add AtomicOperations interface checks
4. [ ] Update error handling to use typed errors

### Phase 3: Replace Usage (Day 2-3)

1. [ ] Update exec_memory_operation.go to use service
2. [ ] Update memory/uc files one by one
3. [ ] Ensure all tests pass
4. [ ] Run integration tests

## Code Changes Summary

### 1. Update Service Constructor

```go
func NewMemoryOperationsService(
    memoryManager memcore.ManagerInterface,
    templateEngine *tplengine.TemplateEngine,
    lockManager core.LockManager, // Add parameter
) MemoryOperationsService {
    return &memoryOperationsService{
        memoryManager:  memoryManager,
        templateEngine: templateEngine,
        lockManager:    lockManager,
    }
}
```

### 2. Import Existing Packages

```go
import (
    // ... existing imports
    valutil "github.com/compozy/compozy/engine/memory/uc"
    "github.com/compozy/compozy/engine/memory/metrics"
    corerrors "github.com/compozy/compozy/engine/memory/core"
)
```

### 3. Update Validation Calls

```go
// Replace all ValidateMemoryRef with:
if err := valutil.ValidateMemoryRef(req.MemoryRef); err != nil {
    return corerrors.NewMemoryError(corerrors.ErrCodeInvalidInput, err.Error(), nil)
}
```

## Testing Strategy

1. **Unit Tests**: Update existing tests for new behavior
2. **Integration Tests**: Test with lock manager integration
3. **Concurrent Tests**: Add tests for race conditions
4. **Context Tests**: Add cancellation tests

## Rollback Plan

If issues arise during integration:

1. Service is isolated - can revert service changes only
2. Keep old exec_memory_operation.go logic until service is stable
3. Use feature flag if needed for gradual rollout

## Success Criteria

- [ ] All critical issues fixed
- [ ] All existing tests pass
- [ ] No performance regression
- [ ] Successful integration with exec_memory_operation.go
- [ ] At least one memory/uc file migrated successfully
