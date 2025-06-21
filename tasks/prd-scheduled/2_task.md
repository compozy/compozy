---
status: done
---

<task_context>
<domain>engine/workflow/schedule</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>temporal</dependencies>
</task_context>

# Task 2.0: Implement Schedule Manager with Temporal Integration

## Overview

Create the Schedule Manager that performs stateless reconciliation between workflow YAML configurations and Temporal schedules. This is the core component that handles all schedule lifecycle operations and ensures consistency between desired and actual state.

## Subtasks

- [x] 2.1 Define Manager interface and implement base struct

    - Create `engine/workflow/schedule/manager.go` with Manager interface
    - Implement manager struct holding Temporal client and project ID
    - Add constructor function NewManager with proper initialization

- [x] 2.2 Implement ReconcileSchedules function with stateless comparison

    - List all existing schedules from Temporal using project prefix
    - Build desired state map from workflow configurations
    - Compare current vs desired state to identify create/update/delete operations
    - Handle schedule ID generation: `schedule-{project}-{workflow-id}`

- [x] 2.3 Implement Temporal schedule operations (create/update/delete)

    - Map internal Schedule struct to Temporal's schedule.Spec
    - Convert cron expressions and timezone to Temporal format
    - Map OverlapPolicy to Temporal's schedule.Policy
    - Handle jitter, start/end times, and input parameters

- [x] 2.4 Implement worker pool for parallel reconciliation

    - Create worker pool with configurable size (default: 10)
    - Add rate limiting to prevent Temporal API overload
    - Ensure 1000 workflows can be reconciled within 30 seconds
    - Handle context cancellation gracefully

- [x] 2.5 Add comprehensive error handling and retry logic

    - Implement exponential backoff for Temporal API failures
    - Handle specific Temporal errors (ErrScheduleAlreadyRunning, etc.)
    - Log all operations with structured logging
    - Ensure idempotency - multiple runs produce same result

- [x] 2.6 Implement schedule information retrieval methods

    - ListSchedules: Return all schedules with current status
    - GetSchedule: Get specific schedule details from Temporal
    - Include next/last run times and execution status

- [x] 2.7 Write unit tests with mocked Temporal client
    - Test reconciliation with various state combinations
    - Test error scenarios and retry behavior
    - Test performance with large numbers of workflows
    - Verify idempotency of all operations

## Implementation Details

The reconciliation algorithm from the tech spec (lines 110-125):

```go
func (m *Manager) ReconcileSchedules(ctx context.Context, workflows []*workflow.Config) error {
    // 1. Get all schedules from Temporal
    existingSchedules := m.listSchedulesByPrefix(ctx, "schedule-"+m.projectID+"-")

    // 2. Build desired state from YAML
    desiredSchedules := make(map[string]*workflow.Config)
    for _, wf := range workflows {
        if wf.Schedule != nil {
            scheduleID := fmt.Sprintf("schedule-%s-%s", m.projectID, wf.ID)
            desiredSchedules[scheduleID] = wf
        }
    }

    // 3. Reconcile: create/update/delete as needed
    // ... implementation continues
}
```

Key considerations:

- Use sync.WaitGroup or errgroup for parallel operations
- Implement semaphore for rate limiting
- Use structured logging with workflow IDs for debugging

## Success Criteria

- Reconciliation correctly handles all state transitions (create/update/delete)
- Performance meets requirement: 1000 workflows in 30 seconds
- All operations are idempotent and can be safely retried
- Comprehensive error handling prevents partial state
- Unit tests achieve >90% code coverage
- No memory leaks or goroutine leaks

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
