## State Event: `WorkflowExecutionStarted`

**Description:** A new workflow execution has commenced.
**Produced By:** `system.Orchestrator`
**Consumed By:** `state.Manager`, `system.Monitoring`
**Lifecycle Stage:** Start of active execution for a workflow instance.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.started`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:00:05Z",
    "source_component": "system.Orchestrator"
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
      "trigger_input": {
        "user_email": "new.user@example.com",
        "user_name": "Jane Doe"
      },
      "env": {
        "WORKFLOW_WIDE_SETTING": "initial_value_for_instance"
      }
    }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`status`** (`string`, Required)
    -   Description: The initial status of the workflow execution.
-   **`result`** (`object`, Optional)
    -   Description: Holds the result of an operation, if applicable. Not typically used for `Started` events.
-   **`duration_ms`** (`integer`, Optional)
    -   Description: Duration related to the event, if applicable.
-   **`context`** (`object`, Required)
    -   Description: Event-specific contextual data.
