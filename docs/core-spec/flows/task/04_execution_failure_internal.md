# Flow: Task Execution Failure (Internal Error)

This diagram shows a task failing due to an error within the `task.Executor`'s own logic, not related to any external calls.

```mermaid
sequenceDiagram
    participant WO as Workflow Orchestrator
    participant TD as Task Dispatcher
    participant TE as Task Executor
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

    Note over TE: Task logic encounters an unrecoverable internal error

    TE->>NATS: Publishes `TaskExecutionFailed` (State Event)<br>Subject: `compozy.events.task.instance.<task_exec_id>.failed`<br>Payload: {error: {message, code, details}, ...}
    NATS-->>SM: Delivers `TaskExecutionFailed`
    SM-->>SM: Records TaskExecutionFailed
    NATS-->>WO: Delivers `TaskExecutionFailed` (Orchestrator handles task failure)

    Note over WO: Processes task failure (e.g., retry, compensate, fail workflow)
```

This flow shows:
1.  Standard task scheduling and start: `TaskScheduled`, `TaskExecute`, `TaskExecutionStarted`.
2.  The `Task Executor` encounters an internal error during its execution.
3.  The `Task Executor` emits `TaskExecutionFailed` with error details.
4.  The `State Manager` records the failure.
5.  The `Workflow Orchestrator` consumes `TaskExecutionFailed` to take appropriate action (e.g., trigger a retry if configured, execute a compensation path, or mark the workflow as failed). 
