## Command: `TaskExecute`

**Description:** Command to execute a specific task by the task.Dispatcher.
**Produced By:** `task.Dispatcher`
**Consumed By:** `task.Executor` (specific worker that performs the task)
**Lifecycle Stage:** Task is being sent to a worker for actual execution.
**NATS Communication Pattern:** Synchronous (Request-Reply)

### NATS Subject

`compozy.<correlation_id>.task.commands.execute.<task_id>`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "request_id": "<uuid>",
    "source_component": "task.Dispatcher",
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
    "reply_to_subject": "compozy.task.results.<task_exec_id>",
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
-   **`input`** (`object`, Required)
    -   Description: The resolved input data necessary for the task to execute.
-   **`reply_to_subject`** (`string`, Required)
    -   Description: NATS subject where the `TaskExecutionResult` or `TaskFailureResult` should be sent.
-   **`context`** (`object`, Optional)
    -   Description: Additional contextual information for the task execution.