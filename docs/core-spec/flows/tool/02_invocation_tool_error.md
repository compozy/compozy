# Flow: Tool Execution with Tool-Side Error (by Task Executor)

This diagram shows the sequence of events when a `task.Executor` invokes a tool, but the `tool.Executor` (e.g., Deno runtime) encounters an error during the tool's own execution.

```mermaid
sequenceDiagram
    participant TE as Task Executor
    participant ToolExec as Tool Executor (Deno)
    participant SM as State Manager
    participant NATS

    TE->>NATS: Publishes `ToolExecute` (Command)<br>Subject: `compozy.commands.tool.<workflow_id>.execute.<tool_id>`<br>Payload: {tool_id, input_args, reply_to_subject}
    NATS-->>ToolExec: Delivers `ToolExecute`

    Note over ToolExec: Starts tool execution logic

    ToolExec->>NATS: Publishes `ToolExecutionStarted` (State Event)<br>Subject: `compozy.events.tool.instance.<tool_exec_id>.started`
    NATS-->>SM: Delivers `ToolExecutionStarted`
    SM-->>SM: Records ToolExecutionStarted

    Note over ToolExec: Tool logic encounters an internal error

    ToolExec->>NATS: Publishes `ToolExecutionResult` (Command Result)<br>Subject: (reply_to_subject from ToolExecute, e.g., `compozy.results.tool.<tool_exec_id>`)<br>Payload: {status: "TOOL_ERROR", error: {message, code, details}}
    NATS-->>TE: Delivers `ToolExecutionResult`

    Note over TE: Processes tool result indicating a tool error

    TE->>NATS: Publishes `ToolExecutionSuccess` (State Event)<br>Subject: `compozy.events.tool.instance.<tool_exec_id>.completed`<br>Payload: {tool_execution_status: "TOOL_ERROR"}
    NATS-->>SM: Delivers `ToolExecutionSuccess`
    SM-->>SM: Records ToolExecutionSuccess (noting tool_execution_status)
```

This flow involves:
1.  The `Task Executor` sending an `ToolExecute` command.
2.  The `Tool Executor` emitting `ToolExecutionStarted`.
3.  The `Tool Executor`'s internal logic failing, leading it to return a `ToolExecutionResult` with a "TOOL_ERROR" status and error details.
4.  The `Task Executor` receiving this error result and still emitting `ToolExecutionSuccess`, but reflecting the `tool_execution_status` as "TOOL_ERROR". The execution itself (the request-reply part) completed, but the tool's work did not succeed.
5.  The `State Manager` consumes the state events for tracking. 
