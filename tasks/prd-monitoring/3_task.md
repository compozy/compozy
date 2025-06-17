---
status: completed
---

<task_context>
<domain>engine/infra/monitoring</domain>
<type>implementation</type>
<scope>middleware</scope>
<complexity>high</complexity>
<dependencies>temporal</dependencies>
</task_context>

# Task 3.0: Implement Temporal Workflow Metrics

## Overview

Implement Temporal workflow metrics collection using interceptors to track workflow lifecycle events, execution times, and worker counts. This is the most complex task requiring careful handling of various workflow states.

## Subtasks

- [x] 3.1 Create `interceptor/temporal.go` for Temporal interceptor implementation
- [x] 3.2 Define Temporal metrics (workflow_started_total, completed_total, failed_total, duration_seconds, workers_running/configured)
- [x] 3.3 Research Temporal SDK worker state access for accurate running worker count
- [x] 3.4 Implement base `TemporalInterceptor()` structure with ClientInterceptor interface
- [x] 3.5 Add metric recording for successful workflow completion with duration tracking
- [x] 3.6 Add metric recording for workflow failures, ensuring the metric is only labeled by `workflow_type`. Any detailed error categorization should be handled via logging, not metric labels
- [x] 3.7 Handle workflow cancellations and timeouts as distinct metric states
- [x] 3.8 Implement worker count tracking gauges with thread-safe access
- [x] 3.8a Implement 'compozy_temporal_workers_configured_total' gauge to reflect the configured worker count from project settings at startup
- [x] 3.9 Add comprehensive error handling to prevent interceptor failures from affecting workflows
- [x] 3.10 Design hermetic test harness for interceptor without live Temporal server
- [x] 3.11 Create unit tests for each workflow terminal state (success, failure, timeout, cancel)
- [x] 3.12 Add integration tests with test Temporal server for validation

## Implementation Details

### Metric Definitions

Based on the tech spec (lines 506-511), implement these metrics:

```go
// In interceptor/temporal.go
var (
    workflowStartedTotal    metric.Int64Counter
    workflowCompletedTotal  metric.Int64Counter
    workflowFailedTotal     metric.Int64Counter
    workflowDuration        metric.Float64Histogram
    workersRunning         metric.Int64UpDownCounter
    workersConfigured      metric.Int64Gauge
    initOnce               sync.Once
)

func initMetrics(meter metric.Meter) {
    initOnce.Do(func() {
        workflowStartedTotal, _ = meter.Int64Counter(
            "compozy_temporal_workflow_started_total",
            metric.WithDescription("Started workflows"),
        )
        workflowCompletedTotal, _ = meter.Int64Counter(
            "compozy_temporal_workflow_completed_total",
            metric.WithDescription("Completed workflows"),
        )
        workflowFailedTotal, _ = meter.Int64Counter(
            "compozy_temporal_workflow_failed_total",
            metric.WithDescription("Failed workflows"),
        )
        workflowDuration, _ = meter.Float64Histogram(
            "compozy_temporal_workflow_duration_seconds",
            metric.WithDescription("Workflow execution time"),
            metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10),
        )
        workersRunning, _ = meter.Int64UpDownCounter(
            "compozy_temporal_workers_running_total",
            metric.WithDescription("Currently running workers"),
        )
        workersConfigured, _ = meter.Int64Gauge(
            "compozy_temporal_workers_configured_total",
            metric.WithDescription("Configured workers per instance"),
        )
    })
}
```

### Label Requirements

From the allow-list (lines 55-59):

- Temporal metrics can only use: `workflow_type`
- NO error details or reasons in labels (per task 3.6 clarification)

### Interceptor Implementation

Key requirements from the tech spec:

1. **Interface Design** (lines 117-120):

```go
// TemporalInterceptor returns a Temporal interceptor for workflow metrics.
func (m *MonitoringService) TemporalInterceptor(ctx context.Context) (interceptor.ClientInterceptor, error) {
    // Handles workflow lifecycle events
}
```

2. **Integration Pattern** (lines 243-252):

```go
// In engine/worker package
interceptor, err := ms.TemporalInterceptor(ctx)
if err != nil {
    log.Error("Failed to create Temporal interceptor", "error", err)
    // Continue without interceptor rather than failing
}
if interceptor != nil {
    workerOptions.Interceptors = append(workerOptions.Interceptors, interceptor)
}
```

### Workflow Lifecycle Handling

The interceptor must track these distinct states:

1. **Started**: When workflow execution begins
2. **Completed**: Successful workflow completion
3. **Failed**: Workflow failed with error
4. **Cancelled**: Workflow was cancelled
5. **Timed Out**: Workflow exceeded timeout

Each state needs appropriate metric recording:

```go
type workflowInterceptor struct {
    meter metric.Meter
}

func (w *workflowInterceptor) ExecuteWorkflow(
    ctx workflow.Context,
    in *interceptor.ExecuteWorkflowInput,
) (*interceptor.ExecuteWorkflowOutput, error) {
    startTime := time.Now()
    workflowType := in.WorkflowType

    // Record workflow started
    workflowStartedTotal.Add(ctx, 1,
        metric.WithAttributes(attribute.String("workflow_type", workflowType)))

    // Execute the workflow
    result, err := in.Next.ExecuteWorkflow(ctx, in)

    // Record duration
    duration := time.Since(startTime).Seconds()
    workflowDuration.Record(ctx, duration,
        metric.WithAttributes(attribute.String("workflow_type", workflowType)))

    // Handle terminal states
    if err != nil {
        // Determine error type
        switch {
        case temporal.IsCanceledError(err):
            // Handle cancellation separately if needed
        case temporal.IsTimeoutError(err):
            // Handle timeout separately if needed
        default:
            // Generic failure
        }
        workflowFailedTotal.Add(ctx, 1,
            metric.WithAttributes(attribute.String("workflow_type", workflowType)))
    } else {
        workflowCompletedTotal.Add(ctx, 1,
            metric.WithAttributes(attribute.String("workflow_type", workflowType)))
    }

    return result, err
}
```

### Worker Metrics

1. **Configured Workers**: Static count from configuration
2. **Running Workers**: Dynamic count requiring SDK investigation

Research needed (task 3.3):

- Check if Temporal SDK exposes worker pool state
- Investigate thread-safe access patterns
- Consider alternative approaches if not available

### Error Handling Requirements

From lines 264-267:

- Interceptor errors must not abort workflows
- Catch all instrumentation errors
- Log errors via `pkg/logger.Error`
- Use defer/recover for panic protection

### Testing Strategy

1. **Hermetic Testing** (task 3.10):

    - Mock Temporal workflow context
    - Simulate different workflow outcomes
    - Verify metric recording without real server

2. **Unit Tests** (task 3.11):

    - Test each terminal state separately
    - Verify correct metric increments
    - Test error recovery

3. **Integration Tests** (task 3.12):
    - Use Temporal test server
    - Execute real workflows
    - Validate end-to-end metrics

### Complex Implementation Challenges

1. **Worker Count Tracking**:

    - May require custom worker wrapper
    - Need thread-safe increment/decrement
    - Handle worker lifecycle events

2. **Error Classification**:

    - Distinguish timeout vs cancellation vs failure
    - Log detailed errors while keeping metrics simple
    - Maintain low cardinality

3. **Test Harness Design**:
    - Create mock workflow.Context
    - Simulate interceptor chain
    - Provide deterministic outcomes

## Success Criteria

- Temporal interceptor properly integrated with workflow lifecycle
- All workflow metrics (started, completed, failed, duration) are collected
- Worker metrics track running and configured worker counts
- Interceptor handles errors gracefully without disrupting workflows
- Comprehensive test suite covers all workflow states and edge cases
- Performance impact is negligible on workflow execution

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
- **MUST** run `make lint` and `make test-all` before completing ANY subtask
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks
**Enforcement:** Violating these standards results in immediate task rejection.
</critical>
