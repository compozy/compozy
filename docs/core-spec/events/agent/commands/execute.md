## Command: `AgentExecute`

**Description:** Command to execute an agent.
**Produced By:** `task.Executor`
**Consumed By:** `system.Runtime`
**Lifecycle Stage:** Request for agent processing is initiated.
**NATS Communication Pattern:** Synchronous (Request-Reply)

### NATS Subject

`compozy.<correlation_id>.agent.commands.<agent_exec_id>.execute` 

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "request_id": "<uuid>", 
    "event_timestamp": "2025-05-13T20:07:00Z",
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
    "exec_id": "<uuid>" 
  },
  "payload": {
    "context": {
      "input": {
        "customer_inquiry": "I need help with my order ORD123."
      },
      "env": {
        "AGENT_SPECIFIC_VAR": "agent_value"
      },
      "agent_request": {
        // from AgentRequest struct
      }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`context`** (`object`, Optional)
    -   Description: Additional contextual information for the agent execution.
