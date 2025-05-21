## State Event: `WorkflowExecutionCancelled`

**Description:** A workflow execution has been canceled.
**Produced By:** `workflow.Executor`
**Consumed By:** `state.Manager`, `system.Monitoring`
**Lifecycle Stage:** Cancelled termination of a workflow instance.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.canceled`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:02:45Z",
    "source_component": "workflow.Executor"
  },
  "workflow": {
    "id": "user_onboarding_v1",
    "exec_id": "<uuid>"
  },
  "payload": {
    "status": "CANCELED",
    "result": null,
    "duration_ms": 165000,
    "context": {
      "reason": "Manually canceled by administrator"
    }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`status`** (`string`, Required)
    -   Description: The final status of the workflow execution.
-   **`result`** (`object`, Optional)
    -   Description: Holds the result of an operation, if applicable. Not typically used for `Cancelled` events unless there's a specific cancellation output/error.
-   **`duration_ms`** (`integer`, Optional)
    -   Description: The total duration of the workflow execution before it was canceled, in milliseconds.
-   **`context`** (`object`, Optional)
    -   Description: Event-specific contextual data.
