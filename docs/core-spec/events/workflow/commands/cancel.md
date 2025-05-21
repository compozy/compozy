## Command: `WorkflowCancel`

**Description:** A command to cancel a running workflow execution.
**Produced By:** `api.Service`
**Consumed By:** `system.Orchestrator`
**Lifecycle Stage:** Request to terminate a workflow prematurely.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.workflow.cmds.cancel.<workflow_exec_id>`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "source_component": "api.Service",
    "event_timestamp": "2025-01-01T12:00:00Z"
  },
  "workflow": {
    "id": "user_onboarding_v1",
    "exec_id": "<uuid>"
  },
  "payload": {
    "context": {
      "reason": "Manually canceled by administrator",
      "canceled_by": "admin@example.com",
      "canceled_at": "2025-05-13T20:02:40Z"
    }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`context`** (`object`, Required)
    -   Description: Contextual information about the cancellation.
