## State Event: `WorkflowExecutionPaused`

**Description:** A workflow execution has been paused.
**Produced By:** `workflow.Executor`
**Consumed By:** `state.Manager`, `system.Monitoring`
**Lifecycle Stage:** Workflow execution is temporarily suspended.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.workflow.events.<workflow_exec_id>.paused`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:02:00Z",
    "source_component": "workflow.Executor"
  },
  "workflow": {
    "id": "user_onboarding_v1",
    "exec_id": "<uuid>"
  },
  "payload": {
    "status": "PAUSED",
    "result": null,
    "duration_ms": null,
    "context": {
      "reason": "Workflow paused due to external system maintenance.",
      "paused_by": "ops_user_01"
    }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`status`** (`string`, Required)
    -   Description: The current status of the workflow execution.
-   **`result`** (`object`, Optional)
    -   Description: Not typically used for `Paused` events.
-   **`duration_ms`** (`integer`, Optional)
    -   Description: Duration related to the event, if applicable.
-   **`context`** (`object`, Optional)
    -   Description: Event-specific contextual data.
