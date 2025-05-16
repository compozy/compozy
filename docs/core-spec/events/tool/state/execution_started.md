## State Event: `ToolExecutionStarted`

**Description:** The `tool.Executor` (e.g., Deno runtime) has started executing the specific tool function.
**Produced By:** `system.Runtime`
**Consumed By:** `state.Manager`, `system.Monitoring`
**Lifecycle Stage:** Tool begins its execution within the execution environment.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.tool.events.<tool_exec_id>.started`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:07:45Z",
    "source_component": "system.Runtime"
  },
  "workflow": {
    "id": "customer_support_workflow",
    "exec_id": "<workflow_exec_id>"
  },
  "task": {
    "id": "process_customer_inquiry",
    "exec_id": "<task_exec_id>"
  },
  "tool": {
    "id": "order_lookup_tool",
    "exec_id": "<tool_exec_id>"
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
    -   Description: The current status of the tool execution.
-   **`result`** (`object`, Optional)
    -   Description: Holds the result of an operation, if applicable. Not typically used for `Started` events.
-   **`duration_ms`** (`integer`, Optional)
    -   Description: Duration related to the event, if applicable.
-   **`context`** (`object`, Optional)
    -   Description: Event-specific contextual data.
