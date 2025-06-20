# Scheduled Workflows Example

This example demonstrates the basic scheduled workflow functionality in Compozy with a simple workflow that runs every 15 seconds.

## What it tests

- **Cron-based scheduling** with a 15-second interval
- **Schedule management** via REST API
- **Automatic workflow execution** based on schedule

## Cron Format

Compozy supports three schedule formats:

### 1. @every syntax for simple intervals

- `"@every 15s"` - Every 15 seconds
- `"@every 5m"` - Every 5 minutes
- `"@every 1h30m"` - Every 1 hour and 30 minutes
- `"@every 24h"` - Every 24 hours

### 2. 6-field cron format (recommended)

```
┌─────────── second (0 - 59)
│ ┌───────── minute (0 - 59)
│ │ ┌─────── hour (0 - 23)
│ │ │ ┌───── day of month (1 - 31)
│ │ │ │ ┌─── month (1 - 12)
│ │ │ │ │ ┌─ day of week (0 - 6) (Sunday to Saturday)
│ │ │ │ │ │
* * * * * *
```

Examples:

- `"0 * * * * *"` - At 0 seconds of every minute
- `"*/30 * * * * *"` - Every 30 seconds
- `"0 0 * * * *"` - Every hour at 0 minutes 0 seconds
- `"0 0 9 * * 1-5"` - Every weekday at 9:00:00 AM
- `"0,15,30,45 * * * * *"` - At 0, 15, 30, and 45 seconds of every minute

### 3. 7-field cron format (optional)

```
┌─────────── second (0 - 59)
│ ┌───────── minute (0 - 59)
│ │ ┌─────── hour (0 - 23)
│ │ │ ┌───── day of month (1 - 31)
│ │ │ │ ┌─── month (1 - 12)
│ │ │ │ │ ┌─ day of week (0 - 6) (Sunday to Saturday)
│ │ │ │ │ │ ┌─ year (optional)
│ │ │ │ │ │ │
* * * * * * *
```

Example:

- `"0 0 0 1 1 * 2025"` - January 1, 2025 at midnight

**Note:** When using 6-field format, Compozy automatically appends a wildcard year (`*`) when creating schedules in Temporal to comply with its 7-field requirement. The API will return schedules in their original format.

## Workflow structure

```
hello-scheduler (scheduled: 0,15,30,45 * * * * *, UTC)
└── say-hello (basic)
    └── tool: echo_tool → message: "Hello from scheduled workflow!"
```

## Running

```bash
cd examples/schedules
../../compozy dev
```

The workflow will automatically execute every 15 seconds. You can see the executions in the logs.

### Managing Schedule via API

See the `api.http` file for example requests to:

- List schedules
- Get schedule details
- Disable/enable the schedule
- Delete the schedule

## Expected behavior

The workflow will run every 15 seconds and output "Hello from scheduled workflow!" with a timestamp.
