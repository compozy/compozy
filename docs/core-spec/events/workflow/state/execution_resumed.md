## State Event: `WorkflowExecutionResumed`

**Description:** A paused workflow execution has been resumed.
**Produced By:** `workflow.Executor`
**Consumed By:** `state.Manager`, `system.Monitoring`
**Lifecycle Stage:** Workflow execution continues after being paused.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.workflow.events.<workflow_exec_id>.resumed`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_id": "<uuid>",
    "event_timestamp": "2025-05-14T09:00:00Z",
    "source_component": "workflow.Executor"
  },
  "workflow": {
    "id": "user_onboarding_v1",
    "exec_id": "<uuid>"
  },
  "payload": {
    "status": "RUNNING",
    "result": null,
    "duration_ms": null, 
    "context": {
      "reason": "Workflow resumed after external system maintenance completed.",
      "resumed_by": "ops_user_01",
      "resume_data": {
        "additional_info": "System back online"
      }
    }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`status`** (`string`, Required)
    -   Description: The status of the workflow execution after resuming. Typically `"RUNNING"`.
-   **`result`** (`object`, Optional)
    -   Description: Not typically used for `Resumed` events unless resuming directly leads to a final result.
-   **`duration_ms`** (`integer`, Optional)
    -   Description: The duration in milliseconds the workflow was in a paused state.
-   **`context`** (`object`, Optional)
    -   Description: Event-specific contextual data.
