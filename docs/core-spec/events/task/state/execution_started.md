## State Event: `TaskExecutionStarted`

**Description:** A task.Executor has picked up and started executing a task.
**Produced By:** `task.Executor`
**Consumed By:** `state.Manager`, `system.Monitoring`
**Lifecycle Stage:** Task begins active processing by a worker.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.task.events.<task_exec_id>.started`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:00:15Z",
    "source_component": "task.Executor"
  },
  "workflow": {
    "exec_id": "<uuid>"
  },
  "task": {
    "id": "send_welcome_email",
    "exec_id": "<uuid>"
  },
  "payload": {
    "status": "RUNNING",
    "result": null,
    "duration_ms": null,
    "context": {}
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`status`** (`string`, Required)
    -   Description: The current status of the task execution.
-   **`result`** (`object`, Optional)
    -   Description: Holds the result of an operation, if applicable. Not typically used for `Started` events.
-   **`duration_ms`** (`integer`, Optional)
    -   Description: Duration related to the event, if applicable.
-   **`context`** (`object`, Optional)
    -   Description: Event-specific contextual data.
