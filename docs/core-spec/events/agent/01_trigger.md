## Command: `TriggerAgent`

**Description:** Command to trigger an agent to perform an action.
**Produced By:** `task.Executor`
**Consumed By:** `agent.Dispatcher`
**Lifecycle Stage:** Request for agent processing is initiated.
**NATS Communication Pattern:** Synchronous (Request-Reply)

### NATS Subject

`compozy.<correlation_id>.agent.commands.trigger.<agent_id>` 

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
    "reply_to_subject": "compozy.agent.results.<agent_exec_id>",
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
-   **`reply_to_subject`** (`string`, Required)
    -   Description: NATS subject for the `AgentExecutionResult`.
    -   Notes: Pattern: `compozy.agent.results.<agent_exec_id>`.
-   **`context`** (`object`, Optional)
    -   Description: Contextual information for the agent trigger.
