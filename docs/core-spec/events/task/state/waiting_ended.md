## State Event: `TaskWaitingEnd`

**Description:** A Task has exited a waiting state and resumed processing.
**Produced By:** `task.Executor`
**Consumed By:** `state.Manager`, `system.Monitoring`
**Lifecycle Stage:** Task continues after its waiting condition was met or it was externally resumed.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.task.events.<task_exec_id>.waiting_ended`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_id": "<uuid>",
    "event_timestamp": "2025-05-14T15:30:05Z",
    "source_component": "task.Executor"
  },
  "workflow": {
    "id": "user_onboarding_v1",
    "exec_id": "<workflow_exec_id>"
  },
  "task": {
    "id": "await_approval",
    "exec_id": "<task_exec_id>",
    "name": "Await Manager Approval"
  },
  "payload": {
    "status": "RUNNING",
    "result": null,
    "duration_ms": 70200000, 
    "context": {
      "reason": "Approval received for task 'await_approval'",
      "resume_data": {
        "approval_status": "APPROVED",
        "approver_id": "<uuid>"
      }
    }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:

-   **`status`** (`string`, Required)
    -   Description: The status of the task after exiting the waiting state. Typically `"RUNNING"` if processing resumes.
-   **`result`** (`object`, Optional)
    -   Description: Typically `null` for this event, unless resuming directly leads to a final result.
-   **`duration_ms`** (`integer`, Optional)
    -   Description: The total duration in milliseconds the task was in a waiting state.
-   **`context`** (`object`, Required)
    -   Description: Event-specific contextual data.
