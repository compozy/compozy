## State Event: `AgentExecutionStarted`

**Description:** The Agent has received the request and started processing.
**Produced By:** `system.Runtime`
**Consumed By:** `state.Manager`, `system.Monitoring`
**Lifecycle Stage:** Agent begins its internal logic.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.agent.evts.<agent_exec_id>.started`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:07:30Z",
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
    "status": "RUNNING",
    "result": null,
    "duration_ms": null,
    "context": {}
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`status`** (`string`, Required)
    -   Description: The current status of the agent execution.
    -   Example: `"RUNNING"`
    -   Notes: For this event, the status indicates that the agent has started processing.
-   **`result`** (`object`, Optional)
    -   Description: Holds the result of an operation, if applicable. Not typically used for `Started` events.
-   **`duration_ms`** (`integer`, Optional)
    -   Description: Duration related to the event, if applicable.
-   **`context`** (`object`, Optional)
    -   Description: Event-specific contextual data.
