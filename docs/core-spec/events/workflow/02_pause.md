## Command: `WorkflowPause`

**Description:** A command to pause a running workflow execution.
**Produced By:** `api.Layer`, `admin.Interface`, or `external.System`
**Consumed By:** `workflow.Orchestrator`
**Lifecycle Stage:** Request to temporarily suspend a workflow.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.workflow.commands.pause.<workflow_exec_id>`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "request_id": "<uuid>",
    "source_component": "api.Layer",
    "event_timestamp": "2025-01-01T12:00:00Z"
  },
  "workflow": {
    "id": "user_onboarding_v1",
    "exec_id": "<uuid>"
  },
  "payload": {
    "context": {
      "reason": "Pausing workflow for scheduled maintenance on a dependent service.",
      "paused_by": "admin@example.com",
      "estimated_duration_ms": 3600000 
    }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`context`** (`object`, Required)
    -   Description: Contextual information about the pause request.