## Command: `TaskExecute`

**Description:** Command to execute a specific task for a workflow after the task state has been initialized by the orchestrator.
**Produced By:** `system.Orchestrator`
**Consumed By:** `task.Executor` 
**Lifecycle Stage:** Task state has been initialized and is being sent to a task executor for actual execution.
**NATS Communication Pattern:** Synchronous (Request-Reply)

### NATS Subject

`compozy.<correlation_id>.task.cmds.<task_exec_id>.execute`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "request_id": "<uuid>",
    "source_component": "system.Orchestrator",
    "event_timestamp": "2025-01-01T12:00:00Z"
  },
  "workflow": {
    "id": "user_onboarding_v1",
    "exec_id": "<uuid>" 
  },
  "task": {
    "id": "send_welcome_email",
    "exec_id": "<uuid>" 
  },
  "payload": {
    "context": {
      "input": {
        "recipient_email": "new.user@example.com",
        "recipient_name": "Jane Doe",
      },
      "env": {
        "TASK_SPECIFIC_VAR": "task_value",
        "OVERRIDE_WORKFLOW_VAR": "overridden_by_task"
      },
    }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`context`** (`object`, Optional)
    -   Description: Additional contextual information for this specific trigger.
