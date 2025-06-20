# Scheduled Workflows

## Overview

Scheduled workflows enable your AI-powered automation to run automatically at specified times without manual intervention. By adding a `schedule` block to your workflow YAML, you can configure workflows to execute on a recurring basis - from simple daily reports to complex time-based orchestrations.

This feature uses Temporal's native scheduling capabilities to ensure reliable, distributed execution with built-in support for timezones, overlap policies, and execution control.

## Quick Start

To schedule a workflow, add a `schedule` block to your workflow YAML:

```yaml
name: daily-report
description: Generate daily sales report
version: 1.0.0

schedule:
    cron: "0 9 * * *" # Run at 9 AM every day
    timezone: "America/New_York"
    enabled: true

tasks:
    - name: generate-report
      type: ai
      agent: report-generator
      input:
          report_type: daily_sales
```

This workflow will automatically execute every day at 9 AM Eastern Time.

## Configuration Reference

### Schedule Block Fields

| Field            | Type      | Required | Default | Description                                      |
| ---------------- | --------- | -------- | ------- | ------------------------------------------------ |
| `cron`           | string    | Yes      | -       | Standard cron expression defining when to run    |
| `timezone`       | string    | No       | UTC     | IANA timezone name (e.g., "America/New_York")    |
| `enabled`        | boolean   | No       | true    | Whether the schedule is active                   |
| `jitter`         | string    | No       | -       | Random delay to add (e.g., "5m" for 0-5 minutes) |
| `overlap_policy` | string    | No       | skip    | How to handle overlapping runs                   |
| `start_at`       | timestamp | No       | -       | When the schedule should start                   |
| `end_at`         | timestamp | No       | -       | When the schedule should stop                    |
| `input`          | object    | No       | -       | Input parameters for scheduled runs              |

### Cron Expression Format

Compozy uses standard 5-field cron expressions:

```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6) (Sunday = 0)
│ │ │ │ │
* * * * *
```

**Common patterns:**

- `0 9 * * *` - Every day at 9 AM
- `*/15 * * * *` - Every 15 minutes
- `0 0 * * 0` - Every Sunday at midnight
- `0 8-17 * * MON-FRI` - Every hour from 8 AM to 5 PM on weekdays
- `0 0 1 * *` - First day of every month at midnight

### Overlap Policies

Control what happens when a scheduled run is still executing when the next run is due:

- **`skip`** (default) - Skip the new run if previous is still running
- **`allow`** - Allow multiple instances to run concurrently
- **`buffer_one`** - Buffer one pending run while current executes
- **`cancel_other`** - Cancel the running instance and start new one

Example:

```yaml
schedule:
    cron: "*/5 * * * *" # Every 5 minutes
    overlap_policy: skip # Skip if still running
```

### Timezone Support

Schedules run in the specified timezone and automatically handle daylight saving time transitions:

```yaml
schedule:
    cron: "0 9 * * MON-FRI" # 9 AM on weekdays
    timezone: "America/New_York" # Eastern Time
```

**Important DST behavior:**

- Spring forward: If scheduled at 2 AM, the run is skipped when clocks jump to 3 AM
- Fall back: Schedules run only once during the repeated hour

### Jitter

Add randomized delay to prevent thundering herd problems when multiple workflows start simultaneously:

```yaml
schedule:
    cron: "0 * * * *" # Every hour
    jitter: "5m" # Random 0-5 minute delay
```

### Time Boundaries

Limit when a schedule is active using start and end times:

```yaml
schedule:
    cron: "0 0 * * *" # Daily at midnight
    start_at: "2024-01-01T00:00:00Z" # Start in 2024
    end_at: "2024-12-31T23:59:59Z" # End after 2024
```

### Input Parameters

Pass custom input to scheduled workflow runs:

```yaml
schedule:
    cron: "0 6 * * *"
    input:
        report_type: "daily"
        recipients:
            - "team@example.com"
        include_charts: true
```

## GitOps Workflow

Scheduled workflows follow GitOps principles - your YAML files are the source of truth:

1. **Add/Update Schedule**: Modify the `schedule` block in your workflow YAML
2. **Commit & Push**: Changes go through your normal code review process
3. **Automatic Sync**: Compozy detects changes and updates Temporal schedules
4. **Continuous Reconciliation**: System ensures Temporal matches your YAML

### How Reconciliation Works

When the Compozy server starts or configuration reloads:

1. Reads all workflow YAML files with `schedule` blocks
2. Lists existing schedules in Temporal
3. Compares desired state (YAML) with actual state (Temporal)
4. Creates new schedules, updates changed ones, deletes removed ones

This process is idempotent - running it multiple times produces the same result.

## API Management

While YAML is the source of truth, you can make temporary runtime adjustments via the REST API:

### List All Schedules

```bash
GET /api/v0/schedules

Response:
{
    "schedules": [
    {
        "workflow_id": "daily-report",
        "schedule_id": "schedule-myproject-daily-report",
        "cron": "0 9 * * *",
        "timezone": "America/New_York",
        "enabled": true,
        "is_override": false,
        "next_run_time": "2024-01-15T14:00:00Z",
        "last_run_time": "2024-01-14T14:00:00Z",
        "last_run_status": "success"
    }
    ]
}
```

### Get Schedule Details

```bash
GET /api/v0/schedules/{workflowId}
```

### Pause/Resume Schedule

```bash
PUT /api/v0/schedules/{workflowId}
Content-Type: application/json

{
    "enabled": false // Pause the schedule
}
```

### Update Cron Expression

```bash
PUT /api/v0/schedules/{workflowId}
Content-Type: application/json

{
    "cron": "0 10 * * *" // Change to 10 AM
}
```

**Important**: API changes are temporary overrides that persist until the next configuration reload. The `is_override` field indicates when the running state differs from YAML.

## Examples

### Daily Data Pipeline

```yaml
name: etl-pipeline
description: Extract, transform, and load sales data

schedule:
    cron: "0 2 * * *" # 2 AM daily
    timezone: "America/Chicago" # Central Time
    overlap_policy: skip # Don't run if previous still running

tasks:
    - name: extract-data
      type: integration
      config:
          source: "sales-db"

    - name: transform-data
      type: ai
      agent: data-transformer

    - name: load-to-warehouse
      type: integration
      config:
          destination: "analytics-warehouse"
```

### Business Hours Monitor

```yaml
name: system-monitor
description: Monitor system health during business hours

schedule:
    cron: "*/30 8-18 * * MON-FRI" # Every 30 min, 8 AM-6 PM weekdays
    timezone: "Europe/London"
    jitter: "2m" # Slight randomization

tasks:
    - name: check-health
      type: ai
      agent: health-monitor

    - name: alert-if-issues
      type: conditional
      condition: "tasks.check-health.output.has_issues"
      task:
          type: integration
          config:
              service: "pagerduty"
```

### Scheduled with End Date

```yaml
name: holiday-campaign
description: Run holiday marketing campaign

schedule:
    cron: "0 10 * * *"
    timezone: "America/New_York"
    start_at: "2024-11-01T00:00:00Z"
    end_at: "2024-12-31T23:59:59Z"
    input:
        campaign_type: "holiday_2024"

tasks:
    - name: generate-content
      type: ai
      agent: marketing-agent
```

## Troubleshooting

### Common Issues

**Schedule not running:**

1. Check if `enabled: true` in YAML
2. Verify cron expression is valid
3. Check timezone is correctly specified
4. Look for validation errors in server logs

**Overlapping runs:**

- Review your `overlap_policy` setting
- Consider if workflow duration exceeds schedule frequency
- Add monitoring to track execution times

**Time zone issues:**

- Always use IANA timezone names (e.g., "America/New_York", not "EST")
- Remember DST transitions may skip or delay runs
- Use UTC for consistent global scheduling

**API overrides reverting:**

- This is expected behavior - YAML is source of truth
- For permanent changes, update the YAML file
- API changes are for temporary operational needs

### Debugging Commands

View schedule details:

```bash
curl http://localhost:8080/api/v0/schedules/my-workflow
```

Check Temporal directly:

```bash
temporal schedule describe --schedule-id schedule-myproject-my-workflow
```

View execution history:

```bash
temporal workflow list --query 'WorkflowType="my-workflow"'
```

### Best Practices

1. **Use descriptive workflow IDs** - They become part of the schedule ID
2. **Set appropriate overlap policies** - Prevent resource exhaustion
3. **Add jitter for multiple schedules** - Avoid thundering herd
4. **Monitor execution times** - Ensure they fit within schedule frequency
5. **Use time boundaries** - For campaign or temporary workflows
6. **Test cron expressions** - Verify they match your intent
7. **Document schedule purpose** - In the workflow description

## FAQ

**Q: Can I schedule a workflow to run just once?**
A: No, schedules are for recurring execution. For one-time execution, trigger the workflow directly via API.

**Q: What happens during server restarts?**
A: Schedules persist in Temporal. Missed runs during downtime depend on your catchup window settings.

**Q: Can I have multiple schedules for one workflow?**
A: No, each workflow can have only one schedule block. Create separate workflows for different schedules.

**Q: How do I handle maintenance windows?**
A: Use the API to temporarily disable schedules, or use time boundaries in your schedule configuration.

**Q: Are seconds supported in cron expressions?**
A: No, Compozy uses 5-field cron (minute precision). For sub-minute scheduling, consider a different approach.

**Q: What's the minimum schedule frequency?**
A: Every minute (`* * * * *`). For higher frequency, trigger workflows through other means.

**Q: How do I test schedule configurations?**
A: Deploy to a development environment first, or use the API to create temporary test schedules.
