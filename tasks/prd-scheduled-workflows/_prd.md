# Product Requirements Document (PRD)

# Scheduled Workflows for Compozy

## Overview

Compozy currently requires manual workflow execution through API calls and direct invocation. However, many AI-powered automation use cases require **time-based scheduling** – from periodic data processing pipelines to scheduled report generation and maintenance tasks.

This PRD defines the requirements for enabling workflows to run automatically based on time-based schedules, allowing users to define scheduling directly in their workflow YAML files.

**Target Users:**

- DevOps teams automating infrastructure tasks
- Data engineers building ETL pipelines with AI components
- Business analysts scheduling recurring reports
- Developers creating maintenance and monitoring workflows

**Value Proposition:** Enable time-based automation of AI workflows directly within workflow YAML configurations, eliminating the need for external schedulers and providing reliable, managed execution of recurring tasks.

## Goals

- **Automation:** Enable fully automated AI agent workflows without manual intervention
- **Simplicity:** Allow users to add scheduling with a simple `schedule` block in their existing workflow YAML
- **Reliability:** Ensure scheduled workflows execute on time with 99.9% accuracy
- **GitOps Compatibility:** Maintain YAML as source of truth with version-controlled scheduling
- **Flexibility:** Support standard cron expressions, timezones, and execution policies

## User Stories

### Primary User Stories

1. **As a developer**, I want to add a `schedule` block to my workflow YAML, so that my workflow runs automatically at specified times without external triggers.

2. **As a DevOps engineer**, I want to schedule nightly infrastructure health checks, so that I can proactively identify issues before they impact users.

3. **As a data analyst**, I want to schedule daily report generation workflows with timezone support, so that reports are delivered at the correct local time.

4. **As a developer**, I want to control overlap behavior when scheduled runs take longer than expected, so that I can prevent resource exhaustion.

5. **As a team lead**, I want to pause and resume scheduled workflows, so that I can handle maintenance windows without losing schedule configuration.

### Edge Cases

6. **As a global team member**, I want my schedules to handle daylight saving time correctly, so that workflows run at consistent wall-clock times year-round.

7. **As a developer**, I want to backfill missed workflow runs after outages, so that data gaps can be filled.

8. **As a platform user**, I want to add jitter to my schedules, so that multiple workflows don't overwhelm the system at the same time.

## Core Features

### 1. Schedule Configuration in YAML

**What it does:** Allows users to define when workflows should execute automatically by adding a `schedule` block to their workflow YAML.

**Why it's important:** Provides a simple, declarative way to enable scheduling without leaving the workflow definition.

**How it works:** Users add a `schedule` section to their existing workflow YAML with timing configuration.

**Functional Requirements:**

1. The system must support standard cron expressions for schedule definition
2. Users must be able to specify timezone for schedule execution (default: UTC)
3. The system must allow optional randomized delay (jitter) to prevent thundering herd
4. Users must be able to define custom input parameters for scheduled runs
5. Schedule definitions must support start and end dates for time-bounded automation
6. The system must validate schedule configuration when workflow is deployed

### 2. Schedule Management

**What it does:** Provides visibility and control over scheduled workflows.

**Why it's important:** Enables users to monitor, pause, resume, and troubleshoot scheduled executions.

**How it works:** Through API endpoints and CLI commands to manage schedule lifecycle.

**Functional Requirements:** 7. Users must be able to list all scheduled workflows in a project 8. The system must show schedule status, last run time, and next run time 9. Users must be able to pause and resume schedules without modifying YAML 10. The system must provide execution history for scheduled runs 11. Users must be able to manually trigger a scheduled workflow outside its schedule 12. The system must support backfilling missed executions within a time window

### 3. Execution Control

**What it does:** Manages how scheduled workflows behave during execution.

**Why it's important:** Prevents resource exhaustion and handles edge cases like long-running workflows.

**How it works:** Configurable policies in the schedule block control execution behavior.

**Functional Requirements:** 13. The system must support overlap policies (skip, allow, buffer, cancel, terminate) 14. Schedules must respect project-level resource quotas and limits 15. The system must track and report when scheduled executions are skipped or delayed 16. Failed scheduled runs must be clearly marked in execution history 17. The system must prevent infinite scheduling loops

### 4. GitOps Integration

**What it does:** Automatically synchronizes schedule definitions from YAML files.

**Why it's important:** Ensures scheduling changes go through version control and code review.

**How it works:** System detects YAML changes and updates active schedules accordingly.

**Functional Requirements:** 18. Schedule changes in YAML must be applied when workflow is redeployed 19. The system must preserve execution history when schedules are updated 20. Removed schedule blocks must disable scheduling for that workflow 21. The system must validate schedule syntax before accepting changes

## User Experience

### Schedule Definition Example

```yaml
id: daily-report
version: "1.0.0"
description: "Generate daily business reports"

# Simple schedule addition to existing workflow
schedule:
    cron: "0 9 * * 1-5" # 9 AM weekdays
    timezone: "America/New_York" # Optional, default UTC
    enabled: true # Optional, default true
    jitter: "5m" # Optional random delay
    overlap_policy: "skip" # Optional, default skip
    input: # Optional default inputs
        report_type: "daily"
        recipients: ["team@company.com"]

# Rest of workflow definition remains unchanged
schemas:
    # ... existing schemas
agents:
    # ... existing agents
tasks:
    # ... existing tasks
```

### Key User Flows

**Adding a Schedule:**

1. User adds `schedule` block to workflow YAML
2. User commits and pushes changes
3. System validates schedule configuration
4. Workflow begins executing on schedule

**Monitoring Schedules:**

1. User runs `compozy schedule list`
2. System shows all scheduled workflows with status
3. User can drill down to see execution history
4. User identifies any failed or skipped runs

### UI/UX Considerations

- Schedule syntax follows familiar cron standard
- Human-readable schedule descriptions in CLI output ("Runs daily at 9 AM EST")
- Clear error messages for invalid cron expressions or timezones
- Execution history shows why runs were skipped (overlap policy, resource limits)

## High-Level Technical Constraints

- Must integrate with existing workflow execution system
- Schedule definitions stored alongside workflow configurations
- Must handle timezone complexities including DST transitions
- Performance must support thousands of scheduled workflows per instance
- Must maintain backward compatibility with non-scheduled workflows

## Non-Goals (Out of Scope)

- Web UI for schedule management (CLI/API only)
- Custom calendar support beyond cron expressions
- Cross-workflow dependencies (workflow A triggers after B)
- Dynamic schedule modification based on workflow results
- Schedule templates or inheritance mechanisms
- Multi-region schedule execution

## Phased Rollout Plan

### MVP (Phase 1) - Basic Scheduling

**Target: 6 weeks**

User-facing capabilities:

- Add `schedule` block to workflow YAML
- Support cron expressions with UTC timezone
- Basic overlap policy (skip duplicate runs)
- View schedule status via CLI/API
- See execution history

Success criteria:

- 10+ beta users successfully using scheduled workflows
- 99% on-time execution rate in testing
- Documentation complete

### Phase 2 - Enhanced Features

**Target: 4 weeks after MVP**

User-facing capabilities:

- Full timezone support with DST handling
- All overlap policies (buffer, cancel, terminate, allow)
- Pause/resume functionality
- Backfill capability for missed runs
- Jitter support

Success criteria:

- 50+ scheduled workflows in production
- 99.5% on-time execution rate
- Positive user feedback on flexibility

### Phase 3 - Scale & Polish

**Target: 4 weeks after Phase 2**

User-facing capabilities:

- Performance optimizations for high scale
- Enhanced monitoring and alerting
- Bulk operations for multiple schedules
- Advanced execution analytics

Success criteria:

- Support 1000+ concurrent schedules
- 99.9% on-time execution rate
- Sub-second schedule operations

## Success Metrics

### User Adoption

- Number of workflows with schedule blocks added
- Percentage of automated vs manual workflow executions
- User retention rate for scheduling feature

### Reliability

- On-time execution rate (target: 99.9%)
- Schedule operation success rate
- Mean time to detect/resolve scheduling issues

### Performance

- Schedule validation time (target: <100ms)
- Time to apply schedule changes (target: <500ms)
- Resource overhead per scheduled workflow (target: <1MB)

### User Satisfaction

- Developer survey on ease of use
- Support ticket volume related to scheduling
- Feature requests and feedback trends

## Risks and Mitigations

### Technical Risks

**Risk:** Timezone and DST handling complexity

- _Impact:_ Incorrect execution times, user confusion
- _Mitigation:_ Use proven timezone libraries, extensive testing, clear documentation

**Risk:** Scale limitations with many concurrent schedules

- _Impact:_ Performance degradation, missed executions
- _Mitigation:_ Load testing, efficient data structures, monitoring

### User Adoption Risks

**Risk:** Users find cron expressions confusing

- _Impact:_ Low adoption, configuration errors
- _Mitigation:_ Provide examples, validation with clear errors, future UI consideration

**Risk:** Migration friction from external schedulers

- _Impact:_ Slow adoption
- _Mitigation:_ Migration guides, superior reliability demonstration

## Open Questions

1. **Default Input Handling:** Should scheduled runs merge default inputs with trigger inputs, or replace entirely?

2. **Schedule Limits:** Should there be limits on scheduling frequency (e.g., minimum 1-minute intervals)?

3. **Monitoring Integration:** Which monitoring platforms should we prioritize for schedule metrics?

4. **Execution Context:** Should scheduled runs have access to previous run outputs?

5. **Cost Model:** How should scheduled executions count toward usage quotas?

## Appendix

### Example Use Cases

**Daily Business Intelligence Report:**

```yaml
schedule:
    cron: "0 9 * * 1-5" # 9 AM weekdays
    timezone: "America/New_York"
    input:
        report_type: "daily"
        include_charts: true
```

**Hourly Data Sync:**

```yaml
schedule:
    cron: "0 * * * *" # Every hour
    overlap_policy: "skip"
    jitter: "5m"
```

**Weekly Cleanup Task:**

```yaml
schedule:
    cron: "0 2 * * 0" # 2 AM every Sunday
    timezone: "UTC"
    overlap_policy: "cancel"
```

### Schedule Configuration Schema

The `schedule` block supports these fields:

- `cron` (required): Standard cron expression
- `timezone` (optional): IANA timezone name, default "UTC"
- `enabled` (optional): Boolean to enable/disable, default true
- `jitter` (optional): Random delay like "5m", "30s"
- `overlap_policy` (optional): "skip", "allow", "buffer", "cancel", "terminate"
- `start_at` (optional): ISO date when schedule begins
- `end_at` (optional): ISO date when schedule ends
- `input` (optional): Default input values for scheduled runs
