## State Event: `AgentDispatchResult`

**Description:** The `agent.Dispatcher` has completed its dispatching of the agent execution.
**Produced By:** `agent.Dispatcher`
**Consumed By:** `state.Manager, task.Executor`
**Lifecycle Stage:** Agent request has been dispatched to a runtime environment.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.agent.events.<agent_exec_id>.{dispatch_success, dispatch_failed}`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:07:05Z",
    "source_component": "agent.Dispatcher",
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
    "status": "{DISPATCH_SUCCESS, DISPATCH_FAILED}",
    "result": {
      // Can be output from the agent or error details
    },
    "duration_ms": null,
    "context": {}
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`status`** (`string`, Required)
    -   Description: Indicates that the dispatch failed or succeeded.
-   **`result`** (`object`, Required)
    -   Description: Details about the dispatch result.
-   **`duration_ms`** (`integer`, Optional)
    -   Description: Duration related to the event, if applicable.
-   **`context`** (`object`, Optional)
    -   Description: Event-specific contextual data.
