## State Event: `AgentExecutionSuccess`

**Description:** The Agent execution has successfully completed.
**Produced By:** `system.Runtime`
**Consumed By:** `state.Manager`, `system.Monitoring`
**Lifecycle Stage:** After the agent execution has been completed successfully.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.agent.evts.<agent_exec_id>.success`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:10:00Z",
    "source_component": "system.Runtime",
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
    "status": "SUCCESS",
    "result": {
      // Output from the agent execution
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
