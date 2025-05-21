## State Event: `ToolExecutionSuccess`

**Description:** The tool execution has successfully completed.
**Produced By:** `system.Runtime`
**Consumed By:** `state.Manager`, `system.Monitoring`
**Lifecycle Stage:** After the tool execution has been completed successfully.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.tool.evts.<tool_exec_id>.success`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:08:00Z",
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
    "status": "SUCCESS", 
    "result": {
      // Output from the tool execution
    },
    "duration_ms": 1000,
    "context": {}
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`status`** (`string`, Required)
    -   Description: The overall status of the tool execution from the caller's perspective. Always "SUCCESS" for this event, meaning the interaction cycle finished.
-   **`result`** (`object`, Optional)
    -   Description: Holds the direct output from the tool if its execution was successful.
-   **`duration_ms`** (`integer`, Optional)
    -   Description: The total duration of the tool execution from the caller's perspective (request to result processing) in milliseconds.
-   **`context`** (`object`, Required)
    -   Description: Event-specific contextual data about the tool's execution result.
