---
status: completed
---

<task_context>
<domain>engine/workflow</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 1.0: Add Schedule Configuration to Workflow Schema

## Overview

Add the Schedule struct to the workflow configuration to enable time-based scheduling capabilities. This foundational task creates the data model and validation logic that all other scheduling features will build upon.

## Subtasks

- [x] 1.1 Define the Schedule struct and OverlapPolicy enum in `engine/workflow/config.go`

    - Add Schedule struct with fields: Cron, Timezone, Enabled, Jitter, OverlapPolicy, StartAt, EndAt, Input
    - Create OverlapPolicy type with constants: skip, allow, buffer_one, cancel_other
    - Add Schedule field to workflow.Config struct

- [x] 1.2 Implement ValidateSchedule function in `engine/workflow/validation.go`

    - Use `robfig/cron/v3` parser for cron expression validation
    - Validate timezone using `time.LoadLocation()`
    - Validate OverlapPolicy against allowed enum values
    - Validate jitter duration format if provided

- [x] 1.3 Integrate Schedule validation into workflow configuration loading

    - Call ValidateSchedule during workflow.Config.Validate()
    - Ensure schedule input is validated against workflow input schema
    - Handle nil/empty schedule blocks gracefully

- [x] 1.4 Write comprehensive unit tests for schedule configuration
    - Test valid cron expressions and timezones
    - Test invalid inputs with clear error messages
    - Test edge cases: DST transitions, invalid timezone names
    - Test default values for optional fields

## Implementation Details

From the tech spec, the Schedule struct should be implemented as:

```go
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
```

The validation function should follow the pattern provided in the tech spec (lines 287-309).

## Success Criteria

- Schedule configuration can be parsed from workflow YAML files
- Validation catches all invalid cron expressions and timezones
- Clear error messages guide users to fix configuration issues
- All tests pass with 100% coverage of validation logic
- No impact on existing workflows without schedule blocks

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
