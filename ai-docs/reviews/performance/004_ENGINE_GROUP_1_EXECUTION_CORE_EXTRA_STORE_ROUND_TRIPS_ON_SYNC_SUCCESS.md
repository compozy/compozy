---
title: "Extra Store Round-Trips on Sync Success"
group: "ENGINE_GROUP_1_EXECUTION_CORE"
category: "performance"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_1_EXECUTION_CORE_PERFORMANCE.md"
issue_index: "4"
sequence: "4"
---

## Extra Store Round-Trips on Sync Success

**Location:** `engine/task/router/exec.go:334–365`, `engine/agent/router/exec.go:149–186`

**Severity:** 🟡 MEDIUM

**Issue:**

```go
// After successful execution:
func buildTaskSyncPayload(ctx context.Context, taskID core.ID, repo Repository) (*SyncPayload, error) {
    // Extra DB call to reload full state
    state, err := repo.GetState(ctx, taskID)
    if err != nil {
        return nil, err
    }

    // Usage already available from execution
    usage, _ := repo.GetUsage(ctx, taskID)

    return &SyncPayload{
        State: state, // Often not needed by client
        Usage: usage,
    }, nil
}
```

**Problem:**

- Clients often only need usage, not full state
- Extra repository round-trip adds 5-20ms latency
- Payload bloat for large states

**Fix:**

```go
// Add query parameter support
// GET /api/v1/tasks/:id/execute?include=state

func buildTaskSyncPayload(c *gin.Context, taskID core.ID, repo Repository, usage *task.UsageSummary) (*SyncPayload, error) {
    includeState := c.Query("include") == "state"

    payload := &SyncPayload{
        Usage: usage,
    }

    if includeState {
        state, err := repo.GetState(c.Request.Context(), taskID)
        if err != nil {
            return nil, err
        }
        payload.State = state
    }

    return payload, nil
}
```

**Impact:**

- Opt-in state loading reduces latency by 10-20%
- Smaller response payloads
- Backward compatible (default includes state if needed)

**Effort:** M (2-3h including both task and agent)  
**Risk:** Low - additive change
