# Flow: Successful Task Execution with Tool Call

This diagram illustrates a task that, as part of its execution, successfully calls a tool.

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

    ToolExec->>NATS: Publishes `ToolExecutionStarted`
    NATS-->>SM: Delivers `ToolExecutionStarted`
    SM-->>SM: Records ToolExecutionStarted

    ToolExec-->>NATS: Publishes `ToolExecutionResult` (SUCCESS) back to TE
    deactivate ToolExec
    NATS-->>TE: Delivers `ToolExecutionResult`
    
    TE->>NATS: Publishes `ToolExecutionSuccess`
    NATS-->>SM: Delivers `ToolExecutionSuccess`
    SM-->>SM: Records ToolExecutionSuccess

    Note over TE: Task logic continues with tool output, then completes

    TE->>NATS: Publishes `TaskExecutionResult`
    NATS-->>SM: Delivers `TaskExecutionResult`
    SM-->>SM: Records TaskExecutionResult
    NATS-->>WO: Delivers `TaskExecutionResult`

    Note over WO: Processes task completion
```

This flow extends the basic task execution by adding a tool call:
1.  Initial steps: `TaskScheduled`, `TaskExecute`, `TaskExecutionStarted` (as in the no-callout flow).
2.  The `Task Executor` determines it needs to call a tool.
3.  **Tool Call Sub-flow (simplified here, see `tool/01_successful_execution.md` for full detail):**
    *   `TE` sends `ToolExecute` to `ToolExecutor`.
    *   `ToolExecutor` emits `ToolExecutionStarted`.
    *   `ToolExecutor` returns `ToolExecutionResult` (success) to `TE`.
    *   `TE` emits `ToolExecutionSuccess`.
4.  The `Task Executor` uses the tool's output and completes its main logic.
5.  The `Task Executor` emits `TaskExecutionResult`.
6.  The `State Manager` and `Workflow Orchestrator` consume events as usual. 
