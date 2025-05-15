## State Event: `WorkflowExecutionTimedOut`

**Description:** A workflow execution has timed out (overall execution timeout, not a waiting timeout).
**Produced By:** `workflow.Orchestrator`
**Consumed By:** `state.Manager`, `monitoring.Systems`, `ui.StreamingLayer`, `alerting.Systems`
**Lifecycle Stage:** Timed out termination of a workflow instance.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.workflow.events.<workflow_exec_id>.timed_out`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:30:00Z",
    "source_component": "workflow.Orchestrator",
    "created_by": "admin_user_01"
  },
  "workflow": {
    "id": "user_onboarding_v1",
    "exec_id": "<uuid>"
  },
  "payload": {
    "status": "TIMED_OUT",
    "result": {
      "error": {
        "message": "Workflow execution exceeded its maximum configured duration.",
        "code": "WORKFLOW_EXECUTION_TIMEOUT"
      }
    },
    "duration_ms": 1800000,
    "context": {
      "timeout_config": {
        "timeout_duration_ms": 1800000,
        "timeout_policy": "TERMINATE"
      },
      "last_activity": {
        "task_id": "<uuid>",
        "task_exec_id": "<uuid>",
        "timestamp": "2025-05-13T20:15:00Z"
      }
    }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`status`** (`string`, Required)
    -   Description: The final status of the workflow execution.
-   **`result`** (`object`, Required)
    -   Description: Details about the timeout error.
-   **`duration_ms`** (`integer`, Required)
    -   Description: The total duration of the workflow execution until it timed out, in milliseconds. This should usually match `context.timeout_config.timeout_duration_ms`.
-   **`context`** (`object`, Optional)
    -   Description: Event-specific contextual data.