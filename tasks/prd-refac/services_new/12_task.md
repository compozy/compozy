# Task 12: Fix Collection Progress Semantic Issue (Not a Race Condition)

STATUS: 🚧 IN PROGRESS

## Problem Statement

The collection task executor is failing with a misleading error message: "collection progress not yet visible for taskExecID X, total children: 3 - retrying" followed by "collection task failed: completed=0, failed=3". This appears to be a race condition but is actually a semantic issue with how "completed" is defined in the codebase.

### Root Cause Analysis

1. **Semantic Confusion**: `CompletedCount` in the codebase means "successfully completed tasks" (StatusSuccess only), not "terminated tasks" (success + failed + canceled + timed_out)
2. **Flawed Race Detection**: The race condition check in `validateAndLogProgress` looks for `CompletedCount == 0 && FailedCount == 0`, which fails when all children fail quickly
3. **Misleading Error Messages**: "completed=0, failed=3" suggests no tasks finished, when actually all 3 failed and are in a terminal state

### Key Code Locations

- `/engine/infra/store/taskrepo.go:519` - Where CompletedCount is set to only StatusSuccess
- `/engine/task/activities/response_helpers.go:65` - The flawed race condition detection
- `/engine/task/progress.go` - The ProgressInfo struct definition

## Solution Approach

Since we're in dev/alpha phase with no backward compatibility requirements, we'll implement a greenfield solution with proper semantics.

### 1. Rename CompletedCount to SuccessCount

**File**: `/engine/task/progress.go`

Update the ProgressInfo struct:

```go
type ProgressInfo struct {
    TotalChildren  int                     `json:"total_children"`
    SuccessCount   int                     `json:"success_count"`      // Renamed from CompletedCount
    FailedCount    int                     `json:"failed_count"`
    CanceledCount  int                     `json:"canceled_count"`     // New field
    TimedOutCount  int                     `json:"timed_out_count"`    // New field
    TerminalCount  int                     `json:"terminal_count"`     // New field
    RunningCount   int                     `json:"running_count"`
    PendingCount   int                     `json:"pending_count"`
    StatusCounts   map[core.StatusType]int `json:"status_counts"`
    CompletionRate float64                 `json:"completion_rate"`
    FailureRate    float64                 `json:"failure_rate"`
    OverallStatus  core.StatusType         `json:"overall_status"`
}
```

### 2. Update GetProgressInfo Logic

**File**: `/engine/infra/store/taskrepo.go`

Replace the confusing incremental updates with clear, declarative assignments:

```go
// Derive specific counters from status counts
progressInfo.SuccessCount = progressInfo.StatusCounts[core.StatusSuccess]
progressInfo.FailedCount = progressInfo.StatusCounts[core.StatusFailed]
progressInfo.CanceledCount = progressInfo.StatusCounts[core.StatusCanceled]
progressInfo.TimedOutCount = progressInfo.StatusCounts[core.StatusTimedOut]
progressInfo.RunningCount = progressInfo.StatusCounts[core.StatusRunning] +
                            progressInfo.StatusCounts[core.StatusWaiting] +
                            progressInfo.StatusCounts[core.StatusPaused]
progressInfo.PendingCount = progressInfo.StatusCounts[core.StatusPending]

// Calculate terminal count
progressInfo.TerminalCount = progressInfo.SuccessCount +
                            progressInfo.FailedCount +
                            progressInfo.CanceledCount +
                            progressInfo.TimedOutCount
```

### 3. Fix Race Condition Detection

**File**: `/engine/task/activities/response_helpers.go`

Update validateAndLogProgress to check for actual race conditions:

```go
// Check if NO children have reached a terminal state
if progressInfo.TerminalCount == 0 && progressInfo.TotalChildren > 0 {
    log.Warn("Progress not yet visible, retrying",
        "parent_id", parentState.TaskExecID,
        "total_children", progressInfo.TotalChildren)
    return fmt.Errorf("%s progress not yet visible for taskExecID %s, total children: %d - retrying",
        expectedType, parentState.TaskExecID, progressInfo.TotalChildren)
}
```

### 4. Update Strategy Calculations

**File**: `/engine/task/progress.go`

Update all references from CompletedCount to SuccessCount and use TerminalCount where appropriate:

- `calculateWaitAllStatus()`
- `calculateFailFastStatus()`
- `calculateBestEffortStatus()`
- `calculateRaceStatus()`
- `IsAllComplete()` - should use TerminalCount

### 5. Fix Child Output Aggregation

**File**: `/engine/task/activities/response_helpers.go:174`

Update to check outputs from all terminal children:

```go
if len(outputsMap) < progressInfo.TerminalCount {
    log.Warn("Child outputs not yet visible, retrying",
        "parent_id", parentState.TaskExecID,
        "expected_terminal", progressInfo.TerminalCount,
        "actual", len(outputsMap))
    return fmt.Errorf("%s child outputs not yet visible for taskExecID %s: have %d, expected %d terminal",
        expectedType, parentState.TaskExecID, len(outputsMap), progressInfo.TerminalCount)
}
```

### 6. Update Error Messages

**File**: `/engine/task/activities/response_helpers.go`

Make error messages clearer in buildDetailedFailureError:

```go
errorMsg.WriteString(fmt.Sprintf("%s task failed: success=%d, failed=%d, canceled=%d, timed_out=%d, terminal=%d/%d, status_counts=%v",
    expectedType, progressInfo.SuccessCount, progressInfo.FailedCount,
    progressInfo.CanceledCount, progressInfo.TimedOutCount,
    progressInfo.TerminalCount, progressInfo.TotalChildren,
    progressInfo.StatusCounts))
```

## Implementation Order

1. Update ProgressInfo struct and add new fields
2. Update GetProgressInfo to populate new fields correctly
3. Fix race condition detection logic
4. Update all strategy calculations
5. Fix child output aggregation
6. Update error messages
7. Search and replace all CompletedCount references throughout codebase
8. Update all tests to use new field names

## Testing

After implementation:

1. Run the failing collection tests to verify they pass
2. Test with collections where all children fail
3. Test with mixed success/failure scenarios
4. Test actual race conditions (very fast child completion)
5. Verify error messages are clear and accurate

## Benefits

- **Clear Semantics**: No confusion between "successful" and "terminal"
- **Accurate Race Detection**: Only triggers on actual race conditions
- **Better Debugging**: Clear error messages showing all status counts
- **Future Proof**: Explicit fields for each terminal state type
