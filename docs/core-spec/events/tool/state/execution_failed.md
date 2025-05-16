## State Event: `ToolExecutionFailed`

**Description:** The tool execution has failed.
**Produced By:** `system.Runtime`
**Consumed By:** `state.Manager`, `system.Monitoring`
**Lifecycle Stage:** After the tool execution has been completed unsuccessfully.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.tool.events.<tool_exec_id>.failed`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:07:55Z",
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
    "status": "FAILED",
    "result": {
      // Error details from the tool execution
    },
    "duration_ms": 30100,
    "context": {}
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`status`** (`string`, Required)
    -   Description: Indicates that the tool execution has failed.
-   **`result`** (`object`, Required)
    -   Description: Details about the error that caused the tool execution to fail.
-   **`duration_ms`** (`integer`, Optional)
    -   Description: The duration of the tool execution attempt in milliseconds.
    -   Example: `30100`
-   **`context`** (`object`, Optional)
    -   Description: Event-specific contextual data.
