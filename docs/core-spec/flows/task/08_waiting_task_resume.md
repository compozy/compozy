# Flow: Waiting Task Resumed

This diagram illustrates a task that enters a waiting state and is later resumed by an external command.

```mermaid
sequenceDiagram
    participant WO as Workflow Orchestrator
    participant TE as Task Executor
    participant ExtSystem as External System/Trigger
    participant SM as State Manager
    participant NATS

    Note over TE: Task logic determines it needs to wait for an external signal
    TE->>NATS: Publishes `TaskWaitingStarted` (State Event)<br>Subject: `compozy.events.task.instance.<task_exec_id>.waiting_started`<br>Payload: {wait_reason: "Waiting for user approval", resume_conditions: {...}}
    NATS-->>SM: Delivers `TaskWaitingStarted`
    SM-->>SM: Records TaskWaitingStarted
    NATS-->>WO: Delivers `TaskWaitingStarted` (Orchestrator may also track waiting tasks)

    Note over TE: Task Executor may release resources or go idle for this task_exec_id

    ...Some time later...

    ExtSystem->>NATS: Publishes `TaskResume` (Command)<br>Subject: `compozy.commands.task.<workflow_id>.resume.<task_exec_id>`<br>Payload: {resume_data: {...}}
    NATS-->>WO: Delivers `TaskResume` (Orchestrator likely handles resume commands)
    
    Note over WO: Processes resume command, identifies the waiting task execution
    WO->>NATS: Publishes `TaskWaitingResumed` (State Event)<br>Subject: `compozy.events.task.instance.<task_exec_id>.waiting_resumed`<br>Payload: {resume_data}
    NATS-->>SM: Delivers `TaskWaitingResumed`
    SM-->>SM: Records TaskWaitingResumed
    
    Note over WO: Orchestrator now needs to re-dispatch or signal the original/a new TE to continue this task execution.
    WO->>NATS: Publishes `TaskExecute` (or similar, with context of resumption)<br>Subject: (Targeting a Task Executor)
    NATS-->>TE: Delivers command to Task Executor

    TE->>NATS: Publishes `TaskExecutionStarted` (or `TaskExecutionResumed` if a distinct event exists)
    NATS-->>SM: Delivers `TaskExecutionStarted` (or Resumed variant)
    SM-->>SM: Records that task has continued

    Note over TE: Task logic continues using resume_data, then completes or fails as usual
    TE->>NATS: Publishes `TaskExecutionResult` / `TaskExecutionFailed`
    NATS-->>SM: (Records)
    NATS-->>WO: (Processes)
```

This flow involves:
1.  A `Task Executor` executing a task, which determines it needs to pause and wait for an external condition or signal.
2.  The `TE` emits `TaskWaitingStarted`. The `Workflow Orchestrator` and `State Manager` are informed.
3.  Later, an `External System` (or user, or another process) issues a `TaskResume` command, targeting the `task_exec_id` of the waiting task.
4.  The `Workflow Orchestrator` (typically) receives this command.
5.  The `WO` emits `TaskWaitingResumed` to record the act of resumption.
6.  The `WO` then re-initiates the execution of the task, possibly by sending an `TaskExecute` command (potentially with additional context about the resumption and any data provided with the resume command) to a `Task Executor` (which could be the original one if stateful, or a new one if the task is stateless and can pick up from where it left off using persisted state).
7.  The `TE` continues the task, emitting `TaskExecutionStarted` (or a more specific `TaskExecutionResumed` event if defined), and eventually `TaskExecutionResult` or `TaskExecutionFailed`. 
