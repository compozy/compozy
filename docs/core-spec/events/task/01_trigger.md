## Command: `TaskTrigger`

**Description:** Command to trigger a specific task execution directly (bypassing workflow execution from the beginning).
**Produced By:** `api.Layer` or `admin.Interface`
**Consumed By:** `task.Dispatcher`
**Lifecycle Stage:** Direct task execution from outside the workflow orchestrator.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.task.commands.trigger.<task_id>`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "request_id": "<uuid>",
    "source_component": "api.Layer",
    "event_timestamp": "2025-01-01T12:00:00Z"
  },
  "workflow": {
    "id": "user_onboarding_v1", 
    "exec_id": "<uuid>" 
  },
  "task": {
    "id": "send_welcome_email"
  },
  "payload": {
    "context": {
      "triggered_by": "admin_user_03",
      "input": {
        "recipient_email": "new.user@example.com",
        "recipient_name": "Jane Doe",
      },
      "env": {
        "TASK_TRIGGER_VAR": "direct_value"
      }
    }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`context`** (`object`, Optional)
    -   Description: Additional contextual information for this specific trigger.