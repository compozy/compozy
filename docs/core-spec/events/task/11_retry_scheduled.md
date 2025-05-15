## State Event: `TaskRetryScheduled`

**Description:** A failed task is being scheduled for a retry.
**Produced By:** `workflow.Orchestrator` or `task.Executor`
**Consumed By:** `state.Manager`, `task.Dispatcher`
**Lifecycle Stage:** Task scheduled for a subsequent attempt after failure.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.task.events.<task_exec_id>.retry_scheduled` 

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:02:30Z",
    "source_component": "workflow.Orchestrator"
  },
  "workflow": {
    "exec_id": "<uuid>"
  },
  "task": {
    "id": "setup_initial_profile",
    "exec_id": "<original_task_exec_id>" 
  },
  "payload": {
    "status": "RETRY_SCHEDULED", 
    "result": null,
    "duration_ms": null, 
    "context": {
      "attempt_number": 2,
      "delay_ms": 5000,
      "retry_policy_active": {
        "max_attempts": 3,
        "backoff_initial": "1s"
      },
      "original_error": {
        "message": "API endpoint returned 503 Service Unavailable",
        "code": "TASK_EXECUTION_ERROR",
        "details": {
          "http_status_code": 503,
          "url": "https://api.example.com/profiles"
        }
      },
      "input": {},
      "env": {}
    }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`status`** (`string`, Required)
    -   Description: Indicates a retry for the task has been scheduled.
-   **`result`** (`object`, Optional)
    -   Description: Holds the result of an operation, if applicable.
-   **`duration_ms`** (`integer`, Optional)
    -   Description: Not typically applicable here, `delay_ms` is in context.
-   **`context`** (`object`, Required)
    -   Description: Event-specific contextual data for the retry.