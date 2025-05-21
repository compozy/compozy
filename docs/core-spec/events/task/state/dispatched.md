## State Event: `TaskDispatched`

**Description:** A task has been dispatched by the system orchestrator to the task executor for execution within a workflow.
**Produced By:** `system.Orchestrator`
**Consumed By:** `state.Manager`, `system.Monitoring`
**Lifecycle Stage:** Task is queued for execution by the task executor.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.task.evts.<task_exec_id>.dispatched`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_id": "<uuid>",
    "event_timestamp": "2025-05-15T17:50:00Z",
    "source_component": "system.Orchestrator",
    "created_by": "system"
  },
  "workflow": {
    "id": "user_onboarding_v1",
    "exec_id": "<uuid>"
  },
  "task": {
    "id": "send_welcome_email",
    "exec_id": "<uuid>",
    "name": "Send Welcome Email"
  },
  "payload": {
    "status": "DISPATCHED",
    "result": null,
    "duration_ms": null,
    "context": {
      "input": {
        "recipient_email": "new.user@example.com",
        "recipient_name": "Jane Doe"
      },
      "env": {
        "TASK_SPECIFIC_VAR": "task_value"
      }
    }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`status`** (`string`, Required)
    -   Description: The status of the task execution.
-   **`result`** (`object`, Optional)
    -   Description: The result of the task execution.
-   **`duration_ms`** (`number`, Optional)
    -   Description: The duration of the task execution in milliseconds.
-   **`context`** (`object`, Optional)
    -   Description: Additional contextual information for this specific trigger.
