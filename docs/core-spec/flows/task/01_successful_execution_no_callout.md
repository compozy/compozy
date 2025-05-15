# Flow: Successful Task Execution (No Callout)

This diagram illustrates a task being scheduled, dispatched, and executed successfully by a `task.Executor` without making any external calls to agents or tools.

```mermaid
sequenceDiagram
    participant WO as Workflow Orchestrator
    participant TD as Task Dispatcher
    participant TE as Task Executor
    participant SM as State Manager
    participant NATS

    Note over WO: Determines next task to run in a workflow
    WO->>NATS: Publishes `TaskScheduled` (State Event)<br>Subject: `compozy.events.task.instance.<task_exec_id>.scheduled`<br>Payload: {workflow_id, workflow_exec_id, task_id, task_exec_id, inputs, ...}
    NATS-->>SM: Delivers `TaskScheduled`
    SM-->>SM: Records TaskScheduled
    NATS-->>TD: Delivers `TaskScheduled` (Task Dispatcher subscribes to these)

    Note over TD: Receives TaskScheduled, prepares to dispatch
    TD->>NATS: Publishes `TaskExecute` (Command)<br>Subject: `compozy.commands.task.<workflow_id>.execute.<task_id>` (or specific TE queue)<br>Payload: {task_exec_id, inputs, ...}
    NATS-->>TE: Delivers `TaskExecute` to an available Task Executor

    Note over TE: Starts task execution logic
    TE->>NATS: Publishes `TaskExecutionStarted` (State Event)<br>Subject: `compozy.events.task.instance.<task_exec_id>.started`
    NATS-->>SM: Delivers `TaskExecutionStarted`
    SM-->>SM: Records TaskExecutionStarted

    Note over TE: Task logic completes successfully (no external calls)

    TE->>NATS: Publishes `TaskExecutionResult` (State Event)<br>Subject: `compozy.events.task.instance.<task_exec_id>.completed`<br>Payload: {outputs, ...}
    NATS-->>SM: Delivers `TaskExecutionResult`
    SM-->>SM: Records TaskExecutionResult
    NATS-->>WO: Delivers `TaskExecutionResult` (Orchestrator subscribes to monitor task completion)

    Note over WO: Processes task completion, decides next step in workflow
```

This flow involves:
1.  The `Workflow Orchestrator` (or a similar component) determines a task needs to run and emits `TaskScheduled`.
2.  The `Task Dispatcher` picks up `TaskScheduled` and issues an `TaskExecute` command to an available `Task Executor`.
3.  The `Task Executor` receives the command and emits `TaskExecutionStarted`.
4.  The `Task Executor` successfully completes the task's logic without calling any external services (Agents or Tools).
5.  The `Task Executor` emits `TaskExecutionResult` with the task's output.
6.  The `State Manager` consumes all state events for tracking.
7.  The `Workflow Orchestrator` also consumes `TaskExecutionResult` to advance the workflow.

*Note: The `TaskExecute` command between Dispatcher and Executor might be an internal mechanism rather than a globally broadcast NATS message, potentially targeting a specific queue for a group of executors.* The `TaskExecutionResult` or `TaskFailureResult` pattern common in some task systems is represented here by the direct emission of `TaskExecutionResult` or `TaskExecutionFailed` by the `Task Executor` for the `State Manager` and `Workflow Orchestrator`. 
