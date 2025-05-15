# Flow: Workflow Cancellation

This diagram illustrates how a running workflow execution is cancelled upon receiving a `WorkflowCancel` command.

```mermaid
sequenceDiagram
    participant Client as Client/API/Trigger
    participant WFEngine as Workflow Engine (Orchestrator)
    participant ActiveTask as Active Task Executor (if any)
    participant SM as State Manager
    participant NATS

    Note over Client: Decides to cancel a running workflow
    Client->>NATS: Publishes `WorkflowCancel` (Command)<br>Subject: `compozy.commands.workflow.cancel.<workflow_exec_id>`<br>Payload: {workflow_exec_id, reason: "User requested cancellation"}
    NATS-->>WFEngine: Delivers `WorkflowCancel`

    Note over WFEngine: Processes cancellation command for <workflow_exec_id>
    WFEngine->>NATS: Publishes `WorkflowExecutionCancelled` (State Event)<br>Subject: `compozy.events.workflow.instance.<workflow_exec_id>.cancelled`<br>Payload: {reason, ...}
    NATS-->>SM: Delivers `WorkflowExecutionCancelled`
    SM-->>SM: Records WorkflowExecutionCancelled
    NATS-->>Client: (Optional) Acknowledge cancellation command processing

    alt If there are active tasks
        Note over WFEngine: Identifies active task(s) for this workflow_exec_id
        WFEngine->>ActiveTask: Sends cancellation signal to active task(s)<br>(e.g., via a specific NATS message like `CancelTaskExecution` or by cancelling context if direct call possible)
        activate ActiveTask
        Note over ActiveTask: Task receives cancellation, attempts to stop gracefully
        ActiveTask->>NATS: Publishes `TaskExecutionCancelled` (or `TaskExecutionFailed` with cancellation error)
        deactivate ActiveTask
        NATS-->>SM: Records task cancellation/failure
        NATS-->>WFEngine: (WFEngine may listen to confirm task cancellations)
    end

```

This flow involves:
1.  A `Client` (or another system component) issues a `WorkflowCancel` command targeting a specific `workflow_exec_id`.
2.  The `Workflow Engine` (WFEngine) receives this command.
3.  The `WFEngine` immediately emits `WorkflowExecutionCancelled` to mark the workflow intent as cancelled. This is important for state tracking, even if cleanup of active tasks takes time.
4.  The `State Manager` records this state.
5.  **Task Cancellation (if applicable):**
    *   The `WFEngine` identifies any currently active tasks belonging to the cancelled workflow execution.
    *   It sends a cancellation instruction to these active tasks (e.g., via a specific NATS command like `CancelTaskExecution` or by other means like cancelling a propagated `context.Context`).
    *   The active tasks, upon receiving the cancellation signal, should attempt to stop gracefully and emit their final state (e.g., `TaskExecutionCancelled` or `TaskExecutionFailed` if cleanup fails).
6.  The `WFEngine` might monitor the result of these task cancellations but the workflow itself is already marked as cancelled. 
