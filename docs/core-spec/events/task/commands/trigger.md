## Command: `TaskTrigger`

**Description:** Command for the `system.Orchestrator` to initiate a specific task execution directly, independently of a full workflow sequence. The API will respond immediately without waiting for the orchestrator to fully process the task trigger or for the task to complete execution.
**Produced By:** `api.Service`
**Consumed By:** `system.Orchestrator`
**Lifecycle Stage:** Direct task execution from outside the system orchestrator.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.task.cmds.<task_id>.trigger`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "source_component": "api.Service",
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
