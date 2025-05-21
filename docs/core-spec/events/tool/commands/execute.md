## Command: `ToolExecute`

**Description:** A command to request the execution of a specific tool. This is typically issued by a Task Executor or an Agent.
**Produced By:** `task.Executor`
**Consumed By:** `system.Runtime`
**Lifecycle Stage:** Request for tool execution is initiated.
**NATS Communication Pattern:** Synchronous (Request-Reply)

### NATS Subject

`compozy.<correlation_id>.tool.cmds.<tool_exec_id>.execute`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:07:00Z",
    "source_component": "task.Executor" 
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
    "context": {
      "input": {
        "order_id": "ORD123"
      },
      "env": {
        "TOOL_API_KEY": "key_from_tool_config_or_task",
        "ANOTHER_VAR": "tool_specific_value"
      },
    }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`context`** (`object`, Required)
    -   Description: The input arguments for the tool, matching its defined input schema.
-   **`context`** (`object`, Optional)
    -   Description: Additional contextual information for the tool execution command.
