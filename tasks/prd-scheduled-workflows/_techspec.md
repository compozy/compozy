# Technical Specification - Scheduled Workflows

## Executive Summary

This specification implements scheduled workflow execution using Temporal's native scheduling. A `schedule:` block in workflow YAML automatically creates Temporal schedules on server startup via stateless reconciliation - comparing YAML definitions against existing Temporal schedules to handle additions, updates, and deletions idempotently. REST API endpoints provide runtime control (pause/resume) as temporary overrides that revert to YAML state on reload, maintaining GitOps as the source of truth.

## System Architecture

### Domain Placement

Components belong to the following `engine/` subdirectories:

- **workflow/** - Schedule configuration struct and validation logic
- **workflow/schedule/** - Schedule manager for Temporal integration
- **infra/server/handlers/** - REST API handlers for schedule management
- **infra/server/router/** - API route registration for schedule endpoints

### Component Overview

**Main Components:**

- **Schedule Config** - YAML configuration model added to workflow.Config
- **Schedule Manager** - Service that creates/updates/deletes Temporal schedules
- **Schedule Handlers** - REST API endpoints for runtime schedule management
- **Temporal ScheduleClient** - Native Temporal client for schedule operations

**Data Flow:**

1. Workflow YAML files with `schedule:` blocks are loaded via LoadProject()
2. Schedule Manager processes workflows and creates Temporal schedules
3. Temporal handles all schedule persistence, execution, and state management
4. REST API provides read/write access to schedule state in Temporal

## Implementation Design

### Core Interfaces

```go
// Schedule configuration in workflow/config.go
type Schedule struct {
    Cron          string                 `yaml:"cron" json:"cron" validate:"required,cron"`
    Timezone      string                 `yaml:"timezone,omitempty" json:"timezone,omitempty"`
    Enabled       *bool                  `yaml:"enabled,omitempty" json:"enabled,omitempty"`
    Jitter        string                 `yaml:"jitter,omitempty" json:"jitter,omitempty"`
    OverlapPolicy OverlapPolicy          `yaml:"overlap_policy,omitempty" json:"overlap_policy,omitempty"`
    StartAt       *time.Time             `yaml:"start_at,omitempty" json:"start_at,omitempty"`
    EndAt         *time.Time             `yaml:"end_at,omitempty" json:"end_at,omitempty"`
    Input         map[string]interface{} `yaml:"input,omitempty" json:"input,omitempty"`
}

// Schedule manager with stateless reconciliation
type Manager interface {
    ReconcileSchedules(ctx context.Context, workflows []*workflow.Config) error
    ListSchedules(ctx context.Context) ([]*ScheduleInfo, error)
    GetSchedule(ctx context.Context, workflowID string) (*ScheduleInfo, error)
    UpdateSchedule(ctx context.Context, workflowID string, update UpdateRequest) error
    DeleteSchedule(ctx context.Context, workflowID string) error
}
```

### Data Models

**Key Types:**

```go
type OverlapPolicy string
const (
    OverlapSkip       OverlapPolicy = "skip"
    OverlapAllow      OverlapPolicy = "allow"
    OverlapBufferOne  OverlapPolicy = "buffer_one"
    OverlapCancelOther OverlapPolicy = "cancel_other"
)

type ScheduleInfo struct {
    WorkflowID    string     `json:"workflow_id"`
    ScheduleID    string     `json:"schedule_id"`
    Cron          string     `json:"cron"`
    Timezone      string     `json:"timezone"`
    Enabled       bool       `json:"enabled"`
    IsOverride    bool       `json:"is_override"`      // API modification
    YAMLConfig    *Schedule  `json:"yaml_config,omitempty"`
    NextRunTime   time.Time  `json:"next_run_time"`
    LastRunTime   *time.Time `json:"last_run_time,omitempty"`
    LastRunStatus string     `json:"last_run_status,omitempty"`
}

type UpdateRequest struct {
    Enabled *bool `json:"enabled"`
}
```

### API Endpoints

**Schedule Management Endpoints:**

- `GET /api/v0/schedules` - List all scheduled workflows with override status
- `GET /api/v0/schedules/{workflowId}` - Get schedule details including YAML divergence
- `PUT /api/v0/schedules/{workflowId}` - Update schedule (temporary override until reload)
    - Request body: `{"enabled": boolean}`
- `DELETE /api/v0/schedules/{workflowId}` - Remove schedule (reverts on reload if in YAML)

## Integration Points

**Temporal Integration:**

- Uses existing Temporal client from worker.Client
- Leverages Temporal's ScheduleClient for all operations
- Schedule IDs follow pattern: `schedule-{project}-{workflow-id}`
- Implements stateless reconciliation on startup

**Reconciliation Algorithm:**

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

## Impact Analysis

| Affected Component           | Type of Impact  | Description & Risk Level                                        | Required Action     |
| ---------------------------- | --------------- | --------------------------------------------------------------- | ------------------- |
| `workflow.Config`            | Schema Addition | Adds optional `Schedule` field. Low risk - backward compatible. | Update validation   |
| `server.setupDependencies()` | Code Addition   | Adds schedule processing after workflow loading. Low risk.      | Add error handling  |
| `WorkflowRepository`         | No Impact       | Schedules stored in Temporal, not database. Zero risk.          | None                |
| REST API Router              | New Routes      | Adds /schedules endpoints. Low risk - new namespace.            | Update OpenAPI docs |
| Temporal Worker              | Indirect Impact | Scheduled workflows increase execution load. Medium risk.       | Monitor capacity    |

**Direct Dependencies:**

- workflow.Config gains Schedule field
- server.mod.go calls Schedule Manager after loading workflows
- New HTTP handlers interact with Temporal ScheduleClient

**Shared Resources:**

- Temporal namespace for schedule storage
- Worker task queue for scheduled executions

## Testing Approach

### Unit Tests

**Components to test:**

- Schedule validation using `robfig/cron/v3` parser
- OverlapPolicy enum validation
- Reconciliation logic (create/update/delete decisions)
- API handler request/response marshaling

**Mock Requirements:**

- Mock Temporal ScheduleClient for all tests
- Mock workflow input validation against schemas

**Critical Test Scenarios:**

- Cron expression edge cases (DST transitions)
- Reconciliation with various state combinations
- API override tracking and reversion
- Concurrent API operations

### Integration Tests

**Test Coverage:**

- Full schedule lifecycle: create → execute → update → delete
- Server restart handling with existing schedules
- YAML changes triggering schedule updates
- Concurrent schedule operations

**Test Data:**

- Sample workflows with various schedule configurations
- Multiple timezone test cases

## Development Sequencing

### Build Order

1. **Schedule Config & Validation** (workflow/config.go)

    - Add Schedule struct
    - Implement validation logic
    - Unit tests for configuration

2. **Schedule Manager** (workflow/schedule/manager.go)

    - Implement Manager interface with reconciliation
    - Stateless comparison of YAML vs Temporal state
    - Validation using robfig/cron/v3 and time.LoadLocation

3. **Server Integration** (infra/server/mod.go)

    - Initialize Schedule Manager in setupDependencies()
    - Run ReconcileSchedules in background goroutine
    - Include reconciliation status in readiness probe

4. **REST API Implementation** (infra/server/handlers/schedule.go)

    - Implement all four endpoints
    - Add to router configuration
    - Update OpenAPI documentation

5. **Testing & Documentation**
    - Comprehensive test suite
    - API documentation
    - User guide for schedule configuration

### Technical Dependencies

- Temporal cluster must be running
- Existing workflow loading mechanism
- Current REST API framework

## Monitoring & Observability

**Metrics to expose:**

```prometheus
# Schedule operation metrics
compozy_schedule_operations_total{operation="create|update|delete",status="success|failure"}
compozy_scheduled_workflows_total{project="",status="active|paused|override"}
compozy_schedule_reconcile_duration_seconds{project=""}
compozy_schedule_reconcile_inflight{project=""}
```

**Key Logs:**

- Reconciliation start/complete with counts (INFO)
- Schedule validation failures with details (ERROR)
- API override operations (WARN)
- Temporal client errors (ERROR)

**Dashboards:**

- Schedule overview: total, active, paused, overridden
- Reconciliation performance and errors
- API operation frequency and latency

## Technical Considerations

### Key Decisions

**Stateless Reconciliation:**

- _Decision:_ Compare YAML state with Temporal state on each startup
- _Rationale:_ Eliminates need for persistent state tracking, handles all cases
- _Trade-off:_ Startup overhead vs operational simplicity

**API Changes as Temporary Overrides:**

- _Decision:_ API modifications revert to YAML state on reload
- _Rationale:_ Maintains GitOps as source of truth while allowing runtime control
- _Implementation:_ Track override status and show divergence in API responses

**Background Processing:**

- _Decision:_ Run reconciliation in goroutine after server startup
- _Rationale:_ Prevents blocking server availability for large workflow sets
- _Trade-off:_ Eventual consistency vs immediate availability

### Known Risks

**Temporal Unavailability at Startup:**

- _Risk:_ Schedules won't be created if Temporal is down during boot
- _Mitigation:_ Retry reconciliation with exponential backoff; log warnings

**DST Transition Edge Cases:**

- _Risk:_ Cron "0 2 \* \* \*" may skip on spring-forward days
- _Mitigation:_ Document timezone best practices; validate with cron library

**Project Rename:**

- _Risk:_ Schedule IDs embed project name, causing orphans on rename
- _Mitigation:_ Provide migration tool or document cleanup procedure

### Special Requirements

**Performance:**

- Reconciliation for 1000 workflows must complete within 30 seconds
- Use worker pool (size=10) with rate limiting for Temporal API calls
- API operations must respond within 100ms

**Security:**

- Schedule API endpoints require same AuthZ as workflow endpoints
- Add `workflow.schedule:read` and `workflow.schedule:write` scopes
- Rate limit: 100 requests/minute per client

**Validation:**

- Cron expressions: `robfig/cron/v3` with standard parser
- Timezones: `time.LoadLocation()` against system tzdata
- Input validation: Against workflow's defined input schema

```go
func ValidateSchedule(cfg *Schedule) error {
    // Validate cron expression
    if _, err := cron.ParseStandard(cfg.Cron); err != nil {
        return fmt.Errorf("invalid cron expression: %w", err)
    }

    // Validate timezone if specified
    if cfg.Timezone != "" {
        if _, err := time.LoadLocation(cfg.Timezone); err != nil {
            return fmt.Errorf("invalid timezone: %w", err)
        }
    }

    // Validate overlap policy
    switch cfg.OverlapPolicy {
    case "", OverlapSkip, OverlapAllow, OverlapBufferOne, OverlapCancelOther:
        // valid
    default:
        return fmt.Errorf("unsupported overlap_policy: %s", cfg.OverlapPolicy)
    }

    return nil
}
```

### Standards Compliance

| Standard                | Compliance | Details                                                     |
| ----------------------- | ---------- | ----------------------------------------------------------- |
| architecture.mdc        | ✓          | Clean Architecture layers, SOLID principles, DRY            |
| go-coding-standards.mdc | ✓          | Error wrapping, small functions, proper documentation       |
| core-libraries.mdc      | ✓          | Uses logger, context propagation, standard error handling   |
| testing-standards.mdc   | ✓          | t.Run("Should..."), testify assertions, mocks for externals |
| api-standards.mdc       | ✓          | RESTful resources, versioned endpoints, consistent errors   |
