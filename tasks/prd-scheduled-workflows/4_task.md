---
status: done
---

<task_context>
<domain>engine/infra/server</domain>
<type>integration</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>temporal</dependencies>
</task_context>

# Task 4.0: Integrate Schedule Reconciliation into Server Startup

## Overview

Modify the server startup process to initialize the Schedule Manager and run reconciliation in a background goroutine. This ensures schedules are synchronized with YAML configurations without blocking server availability.

## Subtasks

- [x] 4.1 Initialize Schedule Manager in setupDependencies

    - Modify engine/infra/server/mod.go setupDependencies function
    - Create Schedule Manager instance with Temporal client
    - Pass project configuration and required dependencies
    - Ensure proper error handling if initialization fails

- [x] 4.2 Launch reconciliation in background goroutine

    - Start ReconcileSchedules after workflows are loaded
    - Run in separate goroutine to prevent blocking server startup
    - Pass appropriate context for cancellation support
    - Log reconciliation start and completion

- [x] 4.3 Implement startup retry with exponential backoff

    - Handle case where Temporal is unavailable at startup
    - Retry initial reconciliation with exponential backoff
    - Max retry duration: 5 minutes with increasing delays
    - Log warnings but don't fail server startup

- [x] 4.4 Integrate with server readiness probe

    - Track initial reconciliation completion status
    - Update readiness probe to include schedule sync status
    - Server remains "not ready" until first sync completes
    - Provide reconciliation status in health endpoint

- [x] 4.5 Handle graceful shutdown
    - Cancel reconciliation context on server shutdown
    - Wait for in-progress reconciliation to complete
    - Add timeout to prevent hanging during shutdown
    - Clean up resources properly

## Implementation Details

Integration point in server startup (based on tech spec):

```go
// In setupDependencies() after loading workflows:
if s.TemporalClient != nil {
    scheduleManager := schedule.NewManager(s.TemporalClient, projectConfig.Name)

    // Create a cancellable context for the background reconciler
    reconcilerCtx, cancelReconciler := context.WithCancel(context.Background())
    // Store cancelReconciler to be called on graceful shutdown
    s.cancelScheduleReconciler = cancelReconciler

    // Start reconciliation in background
    go func() {
        if err := scheduleManager.ReconcileSchedules(reconcilerCtx, workflows); err != nil {
            // Only log error if it's not a context cancellation
            if !errors.Is(err, context.Canceled) {
                log.Error("Schedule reconciliation failed", "error", err)
            }
        }
    }()
}
```

Key considerations:

- Don't use server context for reconciliation to prevent early cancellation
- Track reconciliation state for readiness probe
- Ensure reconciliation errors don't crash the server

## Success Criteria

- Server starts quickly without waiting for reconciliation
- Reconciliation runs reliably in the background
- Readiness probe accurately reflects schedule sync status
- Graceful shutdown handles in-progress reconciliation
- Retry logic handles temporary Temporal unavailability
- No impact on server startup time

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
