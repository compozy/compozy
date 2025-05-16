## State Event: `TaskWaitingStarted`

**Description:** A Task has entered a waiting state, pending an external event, a human action, or another internal process.
**Produced By:** `task.Executor`
**Consumed By:** `state.Manager`, `system.Monitoring`
**Lifecycle Stage:** Task is paused, awaiting a condition to be met.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.task.events.<task_exec_id>.waiting_started`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:00:30Z",
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
    "status": "WAITING",
    "result": null,
    "duration_ms": null,
    "context": {
      "reason": "Human approval required for task 'await_approval'",
      "details": {
        "approver_email": "manager@example.com",
        "approval_reference": "approval_req_789"
      },
      "timeout_config": {
        "timeout_duration_ms": 259200000,
        "timeout_policy": "FAIL_TASK"
      }
    }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`status`** (`string`, Required)
    -   Description: Indicates the task is now in a waiting state.
-   **`result`** (`object`, Optional)
    -   Description: Typically `null` for this event.
-   **`duration_ms`** (`integer`, Optional)
    -   Description: Typically `null` for this event.
-   **`context`** (`object`, Required)
    -   Description: Event-specific contextual data.
