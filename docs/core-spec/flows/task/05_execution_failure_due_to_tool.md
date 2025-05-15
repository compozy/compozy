# Flow: Task Execution Failure (Due to Tool Failure)

This diagram illustrates a task failing because a tool it invoked either returned an error or the execution itself failed (e.g., timeout).

```mermaid
sequenceDiagram
    participant WO as Workflow Orchestrator
    participant TD as Task Dispatcher
    participant TE as Task Executor
    participant ToolExec as Tool Executor
    participant SM as State Manager
    participant NATS

    WO->>NATS: Publishes `TaskScheduled`
    NATS-->>TD: Delivers `TaskScheduled`
    NATS-->>SM: Delivers `TaskScheduled` 
    SM-->>SM: Records TaskScheduled

    TD->>NATS: Publishes `TaskExecute` 
    NATS-->>TE: Delivers `TaskExecute`

    TE->>NATS: Publishes `TaskExecutionStarted`
    NATS-->>SM: Delivers `TaskExecutionStarted`
    SM-->>SM: Records TaskExecutionStarted

    Note over TE: Task logic requires a tool

    TE->>NATS: Publishes `ToolExecute` (Command) to Tool Executor
    NATS-->>ToolExec: Delivers `ToolExecute`
    activate ToolExec

    alt Tool-Side Error (e.g., tool/02_execution_tool_error.md)
        ToolExec->>NATS: Publishes `ToolExecutionStarted` (optional if error is very early)
        NATS-->>SM: Delivers `ToolExecutionStarted`
        SM-->>SM: Records ToolExecutionStarted

        ToolExec-->>NATS: Publishes `ToolExecutionResult` (TOOL_ERROR) back to TE
        NATS-->>TE: Delivers `ToolExecutionResult` {status: "TOOL_ERROR", ...}
        
        TE->>NATS: Publishes `ToolExecutionSuccess` <br>Payload: {tool_execution_status: "TOOL_ERROR"}
        NATS-->>SM: Delivers `ToolExecutionSuccess`
        SM-->>SM: Records ToolExecutionSuccess

    else Execution Timeout/Failure (e.g., tool/03_execution_timeout.md)
        Note over TE: Timeout waiting for ToolExecutionResult
        TE->>NATS: Publishes `ToolExecutionFailed` <br>Payload: {error: {message: "Timeout", ...}}
        NATS-->>SM: Delivers `ToolExecutionFailed`
        SM-->>SM: Records ToolExecutionFailed
    end
    deactivate ToolExec

    Note over TE: Task logic handles the tool failure and decides to fail the task

    TE->>NATS: Publishes `TaskExecutionFailed`<br>Payload: {error: {message: "Task failed due to tool error...", ...}}
    NATS-->>SM: Delivers `TaskExecutionFailed`
    SM-->>SM: Records TaskExecutionFailed
    NATS-->>WO: Delivers `TaskExecutionFailed`

    Note over WO: Processes task failure
```

This flow shows:
1.  Standard task scheduling and start.
2.  The `Task Executor` attempts to call a tool.
3.  **Tool Call Fails (Two common scenarios shown):**
    *   **Tool-Side Error:** The `ToolExecutor` starts, encounters an error, and returns `ToolExecutionResult` with a `TOOL_ERROR` status. The `TaskExecutor` then emits `ToolExecutionSuccess` (reflecting the tool error).
    *   **Execution Timeout/Failure:** The `TaskExecutor` sends `ToolExecute` but times out or encounters a NATS communication issue, leading to `ToolExecutionFailed`.
4.  The `Task Executor`'s logic determines that the tool failure is critical, and thus the task itself must fail.
5.  The `Task Executor` emits `TaskExecutionFailed`.
6.  `State Manager` and `Workflow Orchestrator` process the task failure. 
