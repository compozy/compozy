## State Event: `AgentExecutionFailed`

**Description:** The Agent execution has failed.
**Produced By:** `task.Executor`
**Consumed By:** `state.Manager`, `workflow.Orchestrator`
**Lifecycle Stage:** After the agent execution has been completed unsuccessfully.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.agent.events.<agent_exec_id>.failed`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:10:00Z",
    "source_component": "task.Executor",
    "created_by": "system"
  },
  "workflow": {
    "id": "customer_support_workflow",
    "exec_id": "<uuid>"
  },
  "task": {
    "id": "process_customer_inquiry",
    "exec_id": "<uuid>"
  },
  "agent": {
    "id": "customer_support_agent",
    "exec_id": "<uuid>",
    "name": "Customer Support Agent"
  },
  "payload": {
    "status": "FAILED",
    "result": {
      // Error details from the agent execution
    },
    "duration_ms": 2150,
    "context": {}
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`status`** (`string`, Required)
    -   Description: The final status of the agent execution.
-   **`result`** (`object`, Required)
    -   Description: The output data from the agent execution.
-   **`duration_ms`** (`integer`, Optional)
    -   Description: The total duration of the agent execution in milliseconds.
-   **`context`** (`object`, Optional)
    -   Description: Event-specific contextual data.
