# Schedule Management API

## Overview

The Schedule Management API provides endpoints to view and control scheduled workflows. These endpoints allow you to:

- List all scheduled workflows with their current status
- Get detailed information about specific schedules
- Temporarily override schedule settings (enable/disable, change cron)
- Delete schedules (will be recreated if still defined in YAML)

**Important**: All changes made through these APIs are temporary overrides. The YAML configuration remains the source of truth, and changes will revert to YAML values on the next configuration reload.

## Authentication

All schedule endpoints require authentication using a Bearer token:

```http
Authorization: Bearer <your-token>
```

Required scopes:

- `workflow.schedule:read` - For GET endpoints
- `workflow.schedule:write` - For PUT/PATCH/DELETE endpoints

## Rate Limiting

Schedule endpoints are rate-limited to prevent abuse:

- **Rate limit**: 100 requests per minute per client
- **Headers**: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`

## Endpoints

### List All Schedules

Retrieve a list of all scheduled workflows in the project.

```http
GET /api/v0/schedules
```

#### Response

```json
{
    "success": true,
    "message": "schedules retrieved",
    "data": {
        "schedules": [
            {
                "workflow_id": "daily-report",
                "schedule_id": "schedule-my-project-daily-report",
                "cron": "0 9 * * 1-5",
                "timezone": "America/New_York",
                "enabled": true,
                "is_override": false,
                "next_run_time": "2024-01-15T09:00:00-05:00",
                "last_run_time": "2024-01-14T09:00:00-05:00",
                "last_run_status": "success",
                "yaml_config": {
                    "cron": "0 9 * * 1-5",
                    "timezone": "America/New_York",
                    "enabled": true,
                    "jitter": "5m",
                    "overlap_policy": "skip"
                }
            }
        ],
        "total": 1
    }
}
```

#### Response Fields

| Field             | Type      | Description                                                       |
| ----------------- | --------- | ----------------------------------------------------------------- |
| `workflow_id`     | string    | Unique identifier of the workflow                                 |
| `schedule_id`     | string    | Temporal schedule ID (format: `schedule-{project}-{workflow-id}`) |
| `cron`            | string    | Current cron expression                                           |
| `timezone`        | string    | IANA timezone name                                                |
| `enabled`         | boolean   | Whether the schedule is currently active                          |
| `is_override`     | boolean   | `true` if current state differs from YAML configuration           |
| `next_run_time`   | timestamp | When the workflow will next execute (ISO 8601)                    |
| `last_run_time`   | timestamp | When the workflow last executed (ISO 8601)                        |
| `last_run_status` | string    | Status of the last execution: `success`, `failed`, `running`      |
| `yaml_config`     | object    | Original configuration from YAML file                             |

### Get Schedule Details

Retrieve detailed information about a specific scheduled workflow.

```http
GET /api/v0/schedules/{workflow_id}
```

#### Parameters

| Name          | Type   | Location | Required | Description             |
| ------------- | ------ | -------- | -------- | ----------------------- |
| `workflow_id` | string | path     | Yes      | The workflow identifier |

#### Response

```json
{
    "success": true,
    "message": "schedule retrieved",
    "data": {
        "workflow_id": "daily-report",
        "schedule_id": "schedule-my-project-daily-report",
        "cron": "0 9 * * 1-5",
        "timezone": "America/New_York",
        "enabled": true,
        "is_override": false,
        "next_run_time": "2024-01-15T09:00:00-05:00",
        "last_run_time": "2024-01-14T09:00:00-05:00",
        "last_run_status": "success",
        "yaml_config": {
            "cron": "0 9 * * 1-5",
            "timezone": "America/New_York",
            "enabled": true,
            "jitter": "5m",
            "overlap_policy": "skip",
            "input": {
                "report_type": "daily"
            }
        }
    }
}
```

#### Error Responses

**404 Not Found**

```json
{
    "success": false,
    "error": {
        "code": 404,
        "message": "schedule not found",
        "details": "schedule for workflow 'unknown-workflow' does not exist"
    }
}
```

### Update Schedule

Update a scheduled workflow's settings. This creates a temporary override that persists until the next YAML reload.

```http
PATCH /api/v0/schedules/{workflow_id}
```

#### Parameters

| Name          | Type   | Location | Required | Description             |
| ------------- | ------ | -------- | -------- | ----------------------- |
| `workflow_id` | string | path     | Yes      | The workflow identifier |

#### Request Body

At least one field must be provided:

```json
{
    "enabled": false,
    "cron": "0 10 * * *"
}
```

| Field     | Type    | Required | Description                    |
| --------- | ------- | -------- | ------------------------------ |
| `enabled` | boolean | No\*     | Enable or disable the schedule |
| `cron`    | string  | No\*     | New cron expression            |

\* At least one of `enabled` or `cron` must be provided

#### Response

Returns the updated schedule information:

```json
{
    "success": true,
    "message": "schedule updated",
    "data": {
        "workflow_id": "daily-report",
        "schedule_id": "schedule-my-project-daily-report",
        "cron": "0 10 * * *",
        "timezone": "America/New_York",
        "enabled": false,
        "is_override": true,
        "next_run_time": null,
        "last_run_time": "2024-01-14T09:00:00-05:00",
        "last_run_status": "success",
        "yaml_config": {
            "cron": "0 9 * * 1-5",
            "timezone": "America/New_York",
            "enabled": true
        }
    }
}
```

Note: `is_override` is now `true` indicating the running state differs from YAML.

#### Error Responses

**400 Bad Request**

```json
{
    "success": false,
    "error": {
        "code": 400,
        "message": "at least one of 'enabled' or 'cron' is required",
        "details": "request must contain at least one field to update"
    }
}
```

**400 Bad Request - Invalid Cron**

```json
{
    "success": false,
    "error": {
        "code": 400,
        "message": "invalid cron expression 'invalid cron'",
        "details": "expected exactly 5 fields, found 2: [invalid cron]"
    }
}
```

**404 Not Found**

```json
{
    "success": false,
    "error": {
        "code": 404,
        "message": "schedule not found",
        "details": "schedule for workflow 'unknown-workflow' does not exist"
    }
}
```

### Delete Schedule

Delete a scheduled workflow. If the schedule is still defined in YAML, it will be recreated on the next reconciliation.

```http
DELETE /api/v0/schedules/{workflow_id}
```

#### Parameters

| Name          | Type   | Location | Required | Description             |
| ------------- | ------ | -------- | -------- | ----------------------- |
| `workflow_id` | string | path     | Yes      | The workflow identifier |

#### Response

```json
{
    "success": true,
    "message": "schedule deleted"
}
```

#### Error Responses

**404 Not Found**

```json
{
    "success": false,
    "error": {
        "code": 404,
        "message": "schedule not found",
        "details": "schedule for workflow 'unknown-workflow' does not exist"
    }
}
```

## Example Usage

### Pause a Schedule

```bash
curl -X PATCH https://api.compozy.com/api/v0/schedules/daily-report \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"enabled": false}'
```

### Change Schedule Time

```bash
curl -X PATCH https://api.compozy.com/api/v0/schedules/daily-report \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"cron": "0 10 * * 1-5"}'
```

### Check Override Status

```bash
curl https://api.compozy.com/api/v0/schedules/daily-report \
    -H "Authorization: Bearer ${TOKEN}" \
    | jq '.data.is_override'
```

## SDK Examples

### JavaScript/TypeScript

```typescript
import { CompozyClient } from "@compozy/sdk";

const client = new CompozyClient({ apiKey: "your-api-key" });

// List all schedules
const schedules = await client.schedules.list();
console.log(`Found ${schedules.total} schedules`);

// Get specific schedule
const schedule = await client.schedules.get("daily-report");
if (schedule.isOverride) {
    console.log("Schedule has been modified via API");
}

// Pause a schedule
await client.schedules.update("daily-report", { enabled: false });

// Change cron expression
await client.schedules.update("daily-report", {
    cron: "0 10 * * 1-5",
});
```

### Python

```python
from compozy import CompozyClient

client = CompozyClient(api_key='your-api-key')

# List all schedules
schedules = client.schedules.list()
print(f"Found {schedules['total']} schedules")

# Get specific schedule
schedule = client.schedules.get('daily-report')
if schedule['is_override']:
    print('Schedule has been modified via API')

# Pause a schedule
client.schedules.update('daily-report', enabled=False)

# Change cron expression
client.schedules.update('daily-report', cron='0 10 * * 1-5')
```

### Go

```go
import "github.com/compozy/compozy-go"

client := compozy.NewClient("your-api-key")

// List all schedules
schedules, err := client.Schedules.List(ctx)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Found %d schedules\n", schedules.Total)

// Get specific schedule
schedule, err := client.Schedules.Get(ctx, "daily-report")
if err != nil {
    log.Fatal(err)
}
if schedule.IsOverride {
    fmt.Println("Schedule has been modified via API")
}

// Pause a schedule
enabled := false
_, err = client.Schedules.Update(ctx, "daily-report", &compozy.UpdateScheduleRequest{
    Enabled: &enabled,
})

// Change cron expression
cron := "0 10 * * 1-5"
_, err = client.Schedules.Update(ctx, "daily-report", &compozy.UpdateScheduleRequest{
    Cron: &cron,
})
```

## Error Handling

All endpoints follow standard HTTP status codes and return consistent error responses:

| Status Code | Meaning                                          |
| ----------- | ------------------------------------------------ |
| 200         | Success                                          |
| 400         | Bad Request - Invalid parameters or request body |
| 401         | Unauthorized - Missing or invalid authentication |
| 403         | Forbidden - Insufficient permissions             |
| 404         | Not Found - Schedule does not exist              |
| 429         | Too Many Requests - Rate limit exceeded          |
| 500         | Internal Server Error - Unexpected error         |

Error Response Format:

```json
{
    "success": false,
    "error": {
        "code": 400,
        "message": "Human-readable error message",
        "details": "Technical details about the error",
        "request_id": "req_123456789"
    }
}
```

## Migration Guide

If you're migrating from external schedulers to Compozy's native scheduling:

1. **List External Schedules**: Identify all externally scheduled workflows
2. **Add Schedule Blocks**: Add `schedule:` configuration to workflow YAML files
3. **Deploy Changes**: Push YAML changes through your GitOps pipeline
4. **Verify Schedules**: Use the API to confirm schedules are created
5. **Disable External**: Once verified, disable external schedulers
6. **Monitor**: Use the API to monitor schedule execution

## Best Practices

1. **Check Override Status**: Always check `is_override` to understand if API changes are in effect
2. **Temporary Changes Only**: Remember API changes are temporary - update YAML for permanent changes
3. **Handle 404 Gracefully**: Schedules may not exist if workflows haven't been deployed
4. **Monitor Rate Limits**: Implement exponential backoff when approaching limits
5. **Use Appropriate Scopes**: Request only the permissions your application needs
6. **Validate Cron Expressions**: Test cron expressions before applying them
7. **Consider Timezones**: Be explicit about timezone requirements in your API calls
