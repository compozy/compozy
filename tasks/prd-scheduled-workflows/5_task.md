---
status: done
---

<task_context>
<domain>engine/infra/monitoring</domain>
<type>implementation</type>
<scope>performance</scope>
<complexity>medium</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 5.0: Add Monitoring and Observability for Schedules

## Overview

Implement comprehensive monitoring and observability for the scheduling feature, including Prometheus metrics, structured logging, and Grafana dashboards. This enables operators to track schedule health, performance, and detect issues proactively.

## Subtasks

- [x] 5.1 Define and register Prometheus metrics

    - Create compozy_schedule_operations_total counter
    - Create compozy_scheduled_workflows_total gauge
    - Create compozy_schedule_reconcile_duration_seconds histogram
    - Create compozy_schedule_reconcile_inflight gauge
    - Register metrics with application's Prometheus registry

- [x] 5.2 Instrument Schedule Manager operations

    - Add operation counters for create/update/delete with status labels
    - Record reconciliation duration using histogram
    - Track in-flight reconciliations with gauge
    - Include project label for multi-tenancy support

- [x] 5.3 Add structured logging throughout the system

    - Log reconciliation start/complete with workflow counts
    - Log validation failures with detailed error context
    - Log API override operations as warnings
    - Include workflow_id, schedule_id, and error fields

- [x] 5.4 Instrument API handlers for metrics

    - Update scheduled_workflows_total gauge on API operations
    - Consider background task to periodically refresh gauge
    - Track API operation latency and error rates
    - Maintain low cardinality in metric labels

- [x] 5.5 Create Grafana dashboard and alerts
    - Design dashboard showing schedule overview
    - Display active, paused, and overridden schedules
    - Show reconciliation performance and error rates
    - Configure alerts for reconciliation failures

## Implementation Details

From the tech spec, the required metrics:

```prometheus
# Schedule operation metrics
compozy_schedule_operations_total{operation="create|update|delete",status="success|failure"}
compozy_scheduled_workflows_total{project="",status="active|paused|override"}
compozy_schedule_reconcile_duration_seconds{project=""}
compozy_schedule_reconcile_inflight{project=""}
```

Logging examples:

```go
log.Info("Schedule reconciliation started",
    "project", projectID,
    "workflow_count", len(workflows))

log.Error("Schedule validation failed",
    "workflow_id", workflowID,
    "error", err,
    "cron", schedule.Cron)

log.Warn("Schedule modified via API",
    "workflow_id", workflowID,
    "action", "disable",
    "will_revert_on_reload", true)
```

## Success Criteria

- All specified metrics are properly exposed to Prometheus
- Metrics have appropriate labels without high cardinality
- Logs provide actionable information for troubleshooting
- Grafana dashboard clearly shows system health
- Alerts fire appropriately for critical issues
- Performance impact of instrumentation is minimal (<1%)

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
