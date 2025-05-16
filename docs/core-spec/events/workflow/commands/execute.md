## Command: `WorkflowExecute`

**Description:** An internal command for the `workflow.Executor`'s execution engine to begin processing a previously triggered and acknowledged workflow instance. This typically follows initial validation, persistence of the workflow instance, and, for synchronous triggers, sending an acknowledgment.
**Produced By:** `system.Orchestrator` 
**Consumed By:** `workflow.Executor` 
**Lifecycle Stage:** Orchestrator is ready to start the actual workflow logic (e.g., evaluating the first step or task).
**NATS Communication Pattern:** Asynchronous (internal command)

### NATS Subject

`compozy.<correlation_id>.workflow.commands.<workflow_exec_id>.execute`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid_from_trigger>",
    "request_id": "<uuid_internal_command>",
    "event_timestamp": "2025-05-13T20:00:01Z",
    "source_component": "system.Orchestrator"
  },
  "workflow": {
    "id": "user_onboarding_v1",
    "exec_id": "<uuid_workflow_execution_id>"
  },
  "payload": {
    "context": {
      "initial_input": {
        "user_email": "new.user@example.com",
        "user_name": "Jane Doe"
      },
      "resolved_env": {
        "WORKFLOW_WIDE_SETTING": "initial_value_for_instance"
      },
      "trigger_type": "webhook"
    }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`context`** (`object`, Required)
    -   Description: Contextual information required for the orchestrator to start executing the workflow instance. This typically includes the initial input, resolved environment variables, and other details from the original trigger. 
