## Command: `WorkflowTrigger`

**Description:** A command for the `system.Orchestrator` to trigger a new workflow execution. This is a synchronous command; the requester expects a direct acknowledgment reply containing initial execution details (e.g., `workflow_exec_id`).
**Produced By:** `api.Service`
**Consumed By:** `system.Orchestrator`
**Lifecycle Stage:** Initiation of a new workflow instance, with the orchestrator sending an immediate acknowledgment back to the requester.
**NATS Communication Pattern:** Synchronous (Request-Reply)

### NATS Subject

`compozy.<correlation_id>.workflow.cmds.<workflow_id>.trigger`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "request_id": "<uuid_request_specific>",
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
-   **`context`** (`object`, Required)
    -   Description: Additional contextual information for the workflow execution command.
-   **`context`** (`object`, Optional)
    -   Description: Additional contextual information for the workflow execution command. 
