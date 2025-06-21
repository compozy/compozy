---
status: done
---

<task_context>
<domain>engine/infra/server/handlers</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 3.0: Create REST API for Schedule Management

## Overview

Implement REST API endpoints for schedule management that provide runtime control over scheduled workflows. These endpoints support temporary overrides that persist until the next YAML reload, maintaining GitOps principles while allowing operational flexibility.

## Subtasks

- [x] 3.1 Define API request/response models

    - Create ScheduleInfo struct with IsOverride and YAMLConfig fields
    - Define UpdateRequest struct for PUT endpoint
    - Add proper JSON tags and validation annotations
    - Include fields for tracking API modifications vs YAML state

- [x] 3.2 Implement GET endpoints for schedule listing and details

    - GET /api/v0/schedules - List all schedules with pagination support
    - GET /api/v0/schedules/{workflowId} - Get specific schedule details
    - Compare Temporal state with YAML config to set IsOverride flag
    - Include both current state and original YAML configuration

- [x] 3.3 Implement PUT endpoint for schedule updates

    - PUT /api/v0/schedules/{workflowId} - Update schedule (enable/disable)
    - Validate request body: `{"enabled": boolean}`
    - Apply change to Temporal schedule as temporary override
    - Track modification to show divergence from YAML

- [x] 3.4 Implement DELETE endpoint for schedule removal

    - DELETE /api/v0/schedules/{workflowId} - Remove schedule
    - Delete from Temporal (will be recreated if still in YAML)
    - Return appropriate status codes and error messages
    - Document temporary nature of deletion

- [x] 3.5 Integrate authentication and authorization

    - Add workflow.schedule:read scope for GET endpoints
    - Add workflow.schedule:write scope for PUT/DELETE endpoints
    - Implement rate limiting: 100 requests/minute per client
    - Use existing AuthZ middleware from the project

- [x] 3.6 Update router and API documentation
    - Register new routes in engine/infra/server/router/routes.go
    - Add OpenAPI/Swagger annotations to all endpoints
    - Document request/response formats and error codes
    - Include examples showing IsOverride behavior

## Implementation Details

From the tech spec, the ScheduleInfo response model:

```go
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
```

Key implementation notes:

- State comparison must be efficient - consider caching YAML configs
- Handle race conditions between API calls and background reconciliation
- Clear error messages explaining temporary override behavior

## Success Criteria

- All four endpoints work correctly with proper HTTP status codes
- IsOverride flag accurately reflects divergence from YAML
- Authorization scopes properly enforced
- Rate limiting prevents API abuse
- API documentation is complete and includes examples
- Integration tests verify end-to-end behavior

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
