# Flow: Successful Tool Execution (by Task Executor)

This diagram shows the sequence of events when a `task.Executor` successfully invokes a tool, and the `tool.Executor` (e.g., Deno runtime) successfully executes it.

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

    Note over ToolExec: Tool logic completes successfully

    ToolExec->>NATS: Publishes `ToolExecutionResult` (Command Result)<br>Subject: (reply_to_subject from ToolExecute, e.g., `compozy.results.tool.<tool_exec_id>`)<br>Payload: {status: "SUCCESS", output}
    NATS-->>TE: Delivers `ToolExecutionResult`

    Note over TE: Processes successful tool output

    TE->>NATS: Publishes `ToolExecutionSuccess` (State Event)<br>Subject: `compozy.events.tool.instance.<tool_exec_id>.completed`<br>Payload: {tool_execution_status: "SUCCESS"}
    NATS-->>SM: Delivers `ToolExecutionSuccess`
    SM-->>SM: Records ToolExecutionSuccess
```

This flow involves:
1.  The `Task Executor` sending an `ToolExecute` command.
2.  The `Tool Executor` acknowledging the start of execution by emitting `ToolExecutionStarted`.
3.  The `Tool Executor` returning the `ToolExecutionResult` with a "SUCCESS" status and output.
4.  The `Task Executor`, after processing the result, emitting `ToolExecutionSuccess`.
5.  The `State Manager` consumes the state events for tracking. 
