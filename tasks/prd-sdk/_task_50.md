## status: pending

<task_context>
<domain>sdk/examples</domain>
<type>documentation</type>
<scope>examples</scope>
<complexity>low</complexity>
<dependencies>sdk/schedule</dependencies>
</task_context>

# Task 50.0: Example: Schedules (S)

## Overview

Create example demonstrating workflow scheduling with cron expressions, including different schedule patterns (daily, weekly) and retry configuration.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/05-examples.md (Example 7: Scheduled Workflows)
- **MUST** demonstrate schedule builder with cron
- **MUST** show retry configuration
</critical>

<requirements>
- Runnable example: sdk/examples/07_scheduled_workflow.go
- Demonstrates: ScheduleBuilder with cron expressions
- Shows: Multiple schedule patterns (daily, weekly)
- Retry configuration
- Input mapping for scheduled executions
- Clear comments on cron patterns
</requirements>

## Subtasks

- [ ] 50.1 Create sdk/examples/07_scheduled_workflow.go
- [ ] 50.2 Create workflow for scheduled execution
- [ ] 50.3 Build daily schedule (cron: "0 9 * * *"):
  - [ ] Cron expression
  - [ ] Workflow reference
  - [ ] Input mapping
  - [ ] Retry configuration
- [ ] 50.4 Build weekly schedule (cron: "0 10 * * 1"):
  - [ ] Different cron pattern
  - [ ] Same workflow, different input
  - [ ] Retry configuration
- [ ] 50.5 Build project with schedules
- [ ] 50.6 Add comments explaining cron patterns
- [ ] 50.7 Update README.md with schedule example
- [ ] 50.8 Test example runs successfully

## Implementation Details

Per 05-examples.md section 7:

**Daily schedule:**
```go
dailySchedule, err := schedule.New("daily-report-schedule").
    WithCron("0 9 * * *").
    WithWorkflow("daily-report").
    WithInput(map[string]interface{}{
        "report_type": "daily",
        "include_charts": true,
    }).
    WithRetry(3, 5*time.Minute).
    Build(ctx)
```

**Weekly schedule:**
```go
weeklySchedule, err := schedule.New("weekly-summary-schedule").
    WithCron("0 10 * * 1").
    WithWorkflow("daily-report").
    WithInput(map[string]interface{}{
        "report_type": "weekly",
        "include_charts": true,
    }).
    WithRetry(3, 10*time.Minute).
    Build(ctx)
```

**Project with schedules:**
```go
proj, err := project.New("scheduled-reports").
    AddWorkflow(reportWorkflow).
    AddSchedule(dailySchedule).
    AddSchedule(weeklySchedule).
    Build(ctx)
```

### Relevant Files

- `sdk/examples/07_scheduled_workflow.go` - Main example
- `sdk/examples/README.md` - Updated instructions

### Dependent Files

- `sdk/schedule/builder.go` - ScheduleBuilder
- `sdk/workflow/builder.go` - Workflow for scheduling
- `sdk/project/builder.go` - Project with schedules

## Deliverables

- [ ] sdk/examples/07_scheduled_workflow.go (runnable)
- [ ] Updated README.md with schedule example section
- [ ] Comments explaining:
  - Cron expression syntax
  - Common schedule patterns (daily, weekly, monthly)
  - Retry configuration purpose
  - Input mapping for parameterized schedules
- [ ] Multiple schedule patterns demonstrated
- [ ] Verified example runs successfully

## Tests

From _tests.md:

- Example validation:
  - [ ] Code compiles without errors
  - [ ] Cron expressions validated (valid syntax)
  - [ ] Workflow references validated
  - [ ] Input mapping works
  - [ ] Retry config validated (positive attempts, positive backoff)
  - [ ] Multiple schedules can reference same workflow
  - [ ] Project schedule registration works

## Success Criteria

- Example demonstrates multiple schedule patterns
- Cron syntax clearly explained in comments
- Retry configuration shown
- README updated with schedule usage
- Example runs end-to-end successfully
- Code passes `make lint`
