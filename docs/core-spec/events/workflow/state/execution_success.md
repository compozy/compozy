## State Event: `WorkflowExecutionSuccess`

**Description:** A workflow execution has finished successfully.
**Produced By:** `workflow.Executor`
**Consumed By:** `state.Manager`, `system.Monitoring`
**Lifecycle Stage:** Successful termination of a workflow instance.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.success`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:05:00Z",
    "source_component": "workflow.Executor"
  },
  "workflow": {
    "id": "user_onboarding_v1",
    "exec_id": "<uuid>"
  },
  "payload": {
    "status": "SUCCESS",
    "result": {
      "output": {
        "onboarding_status": "User Jane Doe onboarded successfully.",
        "profile_id": "<uuid>"
      }
    },
    "duration_ms": 300000,
    "context": {}
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`status`** (`string`, Required)
    -   Description: The final status of the workflow execution.
-   **`result`** (`object`, Optional)
    -   Description: The final output data from the workflow execution, if applicable.
-   **`duration_ms`** (`integer`, Optional)
    -   Description: The total duration of the workflow execution in milliseconds.
-   **`context`** (`object`, Optional)
    -   Description: Event-specific contextual data.

