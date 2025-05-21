## State Event: `TaskWaitingTimedOut`

**Description:** A Task in a waiting state has timed out before receiving an external signal or meeting its condition.
**Produced By:** `task.Executor`
**Consumed By:** `state.Manager`, `system.Monitoring`
**Lifecycle Stage:** Task's waiting period exceeded its configured timeout.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.task.evts.<task_exec_id>.waiting_timed_out`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_timestamp": "2025-05-16T20:00:30Z",
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
    "status": "TIMED_OUT",
    "result": {
      // Error details from the task execution
    },
    "duration_ms": 259200000, 
    "context": {
      "reason": "Task timed out waiting for external approval after 3 days.",
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
    -   Description: Indicates the task has timed out while in a waiting state.
-   **`result`** (`object`, Required)
    -   Description: Details about the timeout error.
-   **`duration_ms`** (`integer`, Required)
    -   Description: The duration in milliseconds the task was in the waiting state before timing out. This should generally match `context.timeout_config.timeout_duration_ms`.
-   **`context`** (`object`, Required)
    -   Description: Event-specific contextual data.
