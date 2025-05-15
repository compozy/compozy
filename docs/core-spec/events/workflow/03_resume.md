## Command: `WorkflowResume`

**Description:** A command to resume a workflow execution, typically one that was previously paused or is in a resumable waiting state.
**Produced By:** `api.Layer`, `admin.Interface`, or `external.System`
**Consumed By:** `workflow.Orchestrator`
**Lifecycle Stage:** External signal to continue workflow execution.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.workflow.commands.resume.<workflow_exec_id>`

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
    "resume_data": {
      "override_data": {
        "approval_status": "FORCE_APPROVED",
        "approver_id": "<uuid>"
      }
    },
    "context": {
      "resumed_by": "admin@example.com",
      "resumed_at": "2025-05-14T16:00:00Z",
      "reason": "Manually resumed due to approval timeout"
    }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`resume_data`** (`object`, Optional)
    -   Description: Data provided to resume the workflow.
-   **`context`** (`object`, Optional)
    -   Description: Contextual information about why and how the workflow is being resumed.