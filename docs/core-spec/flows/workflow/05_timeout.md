# Flow: Workflow Timeout

This diagram illustrates a workflow execution timing out based on its defined overall timeout period.

```mermaid
sequenceDiagram
    participant WFEngine as Workflow Engine (Orchestrator/Timer)
    participant ActiveTask as Active Task Executor (if any)
    participant SM as State Manager
    participant NATS

    Note over WFEngine: Workflow <workflow_exec_id> is running.
    Note over WFEngine: Its overall execution timeout is reached.
    Note over WFEngine: (This could be managed by an internal timer in the orchestrator, or an external scheduler that notifies the orchestrator)

    WFEngine->>NATS: Publishes `WorkflowExecutionTimedOut` (State Event)<br>Subject: `compozy.events.workflow.instance.<workflow_exec_id>.timed_out`<br>Payload: {timeout_duration_ms, ...}
    NATS-->>SM: Delivers `WorkflowExecutionTimedOut`
    SM-->>SM: Records WorkflowExecutionTimedOut

    alt If there are active tasks
        Note over WFEngine: Identifies active task(s) for this timed-out workflow
        WFEngine->>ActiveTask: Sends cancellation signal to active task(s)
        activate ActiveTask
        Note over ActiveTask: Task receives cancellation, attempts to stop
        ActiveTask->>NATS: Publishes `TaskExecutionCancelled` / `TaskExecutionFailed`
        deactivate ActiveTask
        NATS-->>SM: Records task result
    end

    Note over WFEngine: Workflow is now considered in a final timed-out state.
```

This flow involves:
1.  A workflow execution is in progress.
2.  The `Workflow Engine` (or an associated timing mechanism) determines that the workflow's maximum allowed execution time has been exceeded.
3.  The `WFEngine` emits `WorkflowExecutionTimedOut`.
4.  The `State Manager` records this terminal state.
5.  **Task Cleanup (if applicable):** Similar to cancellation, the `WFEngine` should attempt to signal any active tasks of this workflow to terminate, as the workflow itself is no longer considered active. 
