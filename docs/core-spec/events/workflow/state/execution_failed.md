## State Event: `WorkflowExecutionFailed`

**Description:** A workflow execution has failed.
**Produced By:** `workflow.Executor`
**Consumed By:** `state.Manager`, `system.Monitoring`
**Lifecycle Stage:** Failed termination of a workflow instance.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.workflow.events.<workflow_exec_id>.failed`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:03:15Z",
    "source_component": "workflow.Executor"
  },
  "workflow": {
    "id": "user_onboarding_v1",
    "exec_id": "<uuid>"
  },
  "payload": {
    "status": "FAILED",
    "result": {
      "error": {
        "code": "TASK_FAILED",
        "message": "Failed to setup initial user profile.",
        "details": {
          "failing_task_id": "<uuid>",
          "failing_task_exec_id": "<uuid>",
          "reason": "API endpoint returned 503 Service Unavailable after 3 retries."
        }
      }
    },
    "duration_ms": 190000,
    "context": {}
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`status`** (`string`, Required)
    -   Description: The final status of the workflow execution.
-   **`result`** (`object`, Required)
    -   Description: Details about the error that caused the workflow execution to fail.
-   **`duration_ms`** (`integer`, Optional)
    -   Description: The total duration of the workflow execution before it failed, in milliseconds.
-   **`context`** (`object`, Optional)
    -   Description: Event-specific contextual data.
