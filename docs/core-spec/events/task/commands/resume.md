## Command: `TaskResume`

**Description:** Command to resume a task that was in a waiting state.
**Produced By:** `api.Service`
**Consumed By:** `task.Executor`
**Lifecycle Stage:** External signal to continue task execution.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.task.cmds.<task_exec_id>.resume`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "request_id": "<uuid>",
    "source_component": "api.Service",
    "event_timestamp": "2025-01-01T12:00:00Z"
  },
  "workflow": {
    "exec_id": "<uuid>"
  },
  "task": {
    "id": "await_approval",
    "exec_id": "<uuid>"
  },
  "payload": {
    "resume_data": {
      "approval_status": "APPROVED",
      "approver_id": "<uuid>",
      "approver_comment": "Looks good, approved."
    },
    "context": {
      "resumed_by": "manager@example.com",
      "resumed_at": "2025-05-14T15:30:00Z"
    }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`resume_data`** (`object`, Required)
    -   Description: Data provided to resume the task, often from an external signal or human interaction.
-   **`context`** (`object`, Optional)
    -   Description: Contextual information about the resume action.
