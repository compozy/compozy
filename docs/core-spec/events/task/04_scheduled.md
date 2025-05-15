## State Event: `TaskScheduled`

**Description:** A task has been scheduled for execution by the workflow.Orchestrator.
**Produced By:** `workflow.Orchestrator`
**Consumed By:** `state.Manager` (for event sourcing)
**Lifecycle Stage:** Task is identified and prepared for execution.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.task.events.<task_exec_id>.scheduled`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:00:10Z",
    "source_component": "workflow.Orchestrator"
  },
  "workflow": {
    "exec_id": "<uuid>"
  },
  "task": {
    "id": "send_welcome_email",
    "exec_id": "<uuid>",
    "name": "Send Welcome Email"
  },
  "payload": {
    "status": "SCHEDULED", 
    "result": null,
    "duration_ms": null,
    "context": {
      "input": {
        "recipient_email": "new.user@example.com",
        "recipient_name": "Jane Doe",
        "template_id": "<uuid>"
      },
      "env": {
        "API_KEY_TASK": "resolved_key"
      },
      "retry_policy": {
        "max_attempts": 3,
        "backoff_initial": "1s"
      }
    }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`status`** (`string`, Required)
    -   Description: Indicates the task has been scheduled.
-   **`result`** (`object`, Optional)
    -   Description: Holds the result of an operation, if applicable. Not typically used for `Scheduled` events.
-   **`duration_ms`** (`integer`, Optional)
    -   Description: Duration related to the event, if applicable.
-   **`context`** (`object`, Required)
    -   Description: Event-specific contextual data for scheduling.