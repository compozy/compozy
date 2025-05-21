## Command: `WorkflowTrigger`

**Description:** A request to start a new workflow execution asynchronously has been received. The API will respond immediately without waiting for initial processing by the orchestrator.
**Produced By:** `api.Service`
**Consumed By:** `system.Orchestrator`
**Lifecycle Stage:** Initiation of a new workflow instance, with immediate API acknowledgment. The orchestrator processes the trigger asynchronously.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.workflow.cmds.<workflow_id>.trigger`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:00:00Z",
    "created_by": "admin_user_01",
    "source_component": "api.Service"
  },
  "workflow": {
    "id": "user_onboarding_v1"
  },
  "payload": {
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
-   **`context`** (`object`, Optional)
    -   Description: Additional contextual information for the workflow trigger command.
