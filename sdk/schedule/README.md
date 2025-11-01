# Schedule Package

The `schedule` package provides a functional options API for creating workflow schedule configurations in Compozy. Schedules define when workflows should be automatically triggered based on cron expressions.

## Features

- **Functional Options Pattern**: Clean, extensible API using functional options
- **Auto-Generated Options**: Options are automatically generated from engine structs
- **Comprehensive Validation**: Validates cron expressions, timezones, and retry policies
- **Type Safety**: Compile-time safety with full Go type checking
- **Deep Copy**: Returns immutable configuration copies

## Installation

```go
import (
    "github.com/compozy/compozy/sdk/schedule"
    engineschedule "github.com/compozy/compozy/engine/project/schedule"
)
```

## Basic Usage

### Minimal Schedule

```go
cfg, err := schedule.New(ctx, "daily-report",
    schedule.WithWorkflowID("generate-report"),
    schedule.WithCron("0 9 * * *"), // Daily at 9 AM
)
if err != nil {
    log.Fatal(err)
}
```

### Schedule with Timezone

```go
cfg, err := schedule.New(ctx, "ny-morning-task",
    schedule.WithWorkflowID("morning-workflow"),
    schedule.WithCron("0 8 * * 1-5"), // Weekdays at 8 AM
    schedule.WithTimezone("America/New_York"),
)
```

### Schedule with Retry Policy

```go
cfg, err := schedule.New(ctx, "critical-task",
    schedule.WithWorkflowID("critical-workflow"),
    schedule.WithCron("0 */4 * * *"), // Every 4 hours
    schedule.WithRetry(&engineschedule.RetryPolicy{
        MaxAttempts: 3,
        Backoff:     time.Minute * 5,
    }),
)
```

### Complete Configuration

```go
enabled := true
cfg, err := schedule.New(ctx, "full-schedule",
    schedule.WithWorkflowID("data-sync"),
    schedule.WithCron("0 2 * * *"), // Daily at 2 AM
    schedule.WithTimezone("UTC"),
    schedule.WithInput(map[string]any{
        "source": "production",
        "target": "warehouse",
    }),
    schedule.WithRetry(&engineschedule.RetryPolicy{
        MaxAttempts: 5,
        Backoff:     time.Minute * 10,
    }),
    schedule.WithEnabled(&enabled),
    schedule.WithDescription("Daily data synchronization"),
)
```

## Available Options

### WithWorkflowID(workflowID string)

Sets the workflow identifier that will be executed when the schedule triggers.

**Required**: Yes

```go
schedule.WithWorkflowID("my-workflow")
```

### WithCron(cron string)

Sets the cron expression that determines when the schedule triggers. Uses standard 5-field cron format.

**Required**: Yes
**Format**: `minute hour day month weekday`

```go
schedule.WithCron("0 9 * * *")      // Daily at 9 AM
schedule.WithCron("*/15 * * * *")   // Every 15 minutes
schedule.WithCron("0 0 * * 0")      // Weekly on Sunday at midnight
schedule.WithCron("0 9-17 * * 1-5") // Weekdays 9 AM to 5 PM
```

### WithTimezone(timezone string)

Sets the IANA timezone name used when evaluating the cron expression.

**Optional**: Defaults to server timezone if not specified
**Format**: IANA timezone (e.g., "America/New_York", "Europe/London", "Asia/Tokyo")

```go
schedule.WithTimezone("America/New_York")
schedule.WithTimezone("Europe/London")
schedule.WithTimezone("UTC")
```

### WithInput(input map[string]any)

Provides default input values supplied to the workflow when triggered.

**Optional**

```go
schedule.WithInput(map[string]any{
    "environment": "production",
    "batchSize": 1000,
})
```

### WithRetry(retry \*engineschedule.RetryPolicy)

Configures retry behavior for failed scheduled executions.

**Optional**
**MaxAttempts**: 1-100
**Backoff**: Positive duration

```go
schedule.WithRetry(&engineschedule.RetryPolicy{
    MaxAttempts: 3,
    Backoff:     time.Minute * 5,
})
```

### WithEnabled(enabled \*bool)

Toggles whether the schedule is active. Use a pointer to distinguish between explicitly disabled and unset.

**Optional**

```go
enabled := true
schedule.WithEnabled(&enabled)

disabled := false
schedule.WithEnabled(&disabled)
```

### WithDescription(description string)

Sets a human-readable description explaining the schedule's purpose.

**Optional**

```go
schedule.WithDescription("Daily report generation for finance team")
```

## Validation Rules

### Schedule ID

- Required
- Must contain only letters, numbers, or hyphens
- Automatically trimmed of whitespace

### Workflow ID

- Required
- Must contain only letters, numbers, or hyphens
- Automatically trimmed of whitespace

### Cron Expression

- Required
- Must be valid 5-field cron expression
- Validated using robfig/cron/v3 standard parser
- Examples:
  - `0 * * * *` - Every hour
  - `*/15 * * * *` - Every 15 minutes
  - `0 9-17 * * 1-5` - Weekdays 9 AM to 5 PM

### Timezone

- Optional (empty string allowed)
- Must be valid IANA timezone if specified
- Validated using time.LoadLocation()
- Common timezones: UTC, America/New_York, Europe/London, Asia/Tokyo

### Retry Policy

- Optional (nil allowed)
- MaxAttempts: Must be between 1 and 100
- Backoff: Must be positive duration

## Error Handling

The constructor returns `*sdkerrors.BuildError` which aggregates all validation errors:

```go
cfg, err := schedule.New(ctx, "",
    schedule.WithWorkflowID("bad id"), // Invalid: contains space
    schedule.WithCron("invalid"),      // Invalid: bad cron syntax
    schedule.WithTimezone("Bad/TZ"),   // Invalid: unknown timezone
)
if err != nil {
    var buildErr *sdkerrors.BuildError
    if errors.As(err, &buildErr) {
        for _, e := range buildErr.Errors {
            log.Printf("Validation error: %v", e)
        }
    }
}
```

## Cron Expression Examples

```go
// Minute-based
"*/5 * * * *"     // Every 5 minutes
"0,30 * * * *"    // Every hour at :00 and :30

// Hour-based
"0 * * * *"       // Every hour
"0 */4 * * *"     // Every 4 hours
"0 9-17 * * *"    // Every hour from 9 AM to 5 PM

// Daily
"0 9 * * *"       // Daily at 9 AM
"0 0 * * *"       // Daily at midnight

// Weekly
"0 9 * * 1"       // Every Monday at 9 AM
"0 0 * * 0"       // Every Sunday at midnight
"0 9 * * 1-5"     // Weekdays at 9 AM

// Monthly
"0 0 1 * *"       // First day of month at midnight
"0 9 15 * *"      // 15th of each month at 9 AM
```

## Code Generation

This package uses code generation to maintain options in sync with engine structs:

```bash
cd sdk/schedule
go generate
```

This regenerates `options_generated.go` from `engine/project/schedule/config.go`.

## Testing

Run tests with:

```bash
gotestsum --format pkgname -- -race -parallel=4 ./sdk/schedule
```

## Migration from Builder Pattern

**Old (Builder Pattern):**

```go
cfg, err := schedule.New("daily-task").
    WithWorkflowID("my-workflow").
    WithCron("0 9 * * *").
    WithTimezone("UTC").
    Build(ctx)
```

**New (Functional Options):**

```go
cfg, err := schedule.New(ctx, "daily-task",
    schedule.WithWorkflowID("my-workflow"),
    schedule.WithCron("0 9 * * *"),
    schedule.WithTimezone("UTC"),
)
```

**Key Changes:**

- `ctx` is now first parameter
- No `.Build(ctx)` call needed
- Options use `schedule.WithX()` prefix
- Validation happens immediately in constructor
