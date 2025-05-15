## Command: `ToolExecute`

**Description:** A command to request the execution of a specific tool. This is typically issued by a Task Executor or an Agent.
**Produced By:** `task.Executor`, `agent.Runtime`
**Consumed By:** `tool.Executor` (e.g., Deno Runtime proxy)
**Lifecycle Stage:** Request for tool execution is initiated.
**NATS Communication Pattern:** Synchronous (Request-Reply)

### NATS Subject

`compozy.<correlation_id>.tool.commands.execute.<tool_id>`

### JSON Payload Example:

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "request_id": "<uuid>",
    "event_timestamp": "2025-05-13T20:07:00Z",
    "source_component": "task.Executor:task_exec_003" 
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
    "reply_to_subject": "compozy.tool.results.<tool_exec_id>",
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
-   **`input_args`** (`object`, Required)
    -   Description: The input arguments for the tool, matching its defined input schema.
-   **`reply_to_subject`** (`string`, Required)
    -   Description: NATS subject where the `ToolExecutionResult` should be sent.
-   **`context`** (`object`, Optional)
    -   Description: Additional contextual information for the tool execution command.