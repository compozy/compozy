# Flow: Task Retry Scheduled

This diagram shows a task failing, and based on its retry policy, the `workflow.Orchestrator` (or a retry-specific component) schedules it for a retry.

```mermaid
sequenceDiagram
    participant WO as Workflow Orchestrator
    participant TD as Task Dispatcher
    participant TE as Task Executor
    participant SM as State Manager
    participant NATS

    TE->>NATS: Publishes `TaskExecutionFailed`<br>Payload: {error: {...}, retry_policy: {max_attempts: 3, current_attempt: 1, ...}}
    NATS-->>SM: Delivers `TaskExecutionFailed`
    SM-->>SM: Records TaskExecutionFailed
    NATS-->>WO: Delivers `TaskExecutionFailed`

    Note over WO: Processes TaskExecutionFailed, checks retry policy
    alt Retry attempt allowed
        WO->>NATS: Publishes `TaskRetryScheduled` (State Event)<br>Subject: `compozy.events.task.instance.<task_exec_id>.retry_scheduled` (or new task_exec_id for retry)
        Payload: {original_task_exec_id, next_attempt_delay_ms, attempt_number: 2, ...}
        NATS-->>SM: Delivers `TaskRetryScheduled`
        SM-->>SM: Records TaskRetryScheduled
        NATS-->>TD: Delivers `TaskRetryScheduled` (Dispatcher may handle scheduling logic for delays)

        Note over TD: If there's a delay, TD waits. Then proceeds to dispatch again (similar to `TaskExecute` in other flows, possibly with updated attempt info).
        TD->>NATS: (After delay, if any) Publishes `TaskExecute` (for retry)
        NATS-->>TE: Delivers `TaskExecute` for retry attempt

    else No more retries
        Note over WO: Max attempts reached or no retry policy. Propagates failure to workflow.
        WO->>NATS: Publishes `WorkflowExecutionFailed` (or similar, depending on overall workflow logic)
    end
```

This flow involves:
1.  A `Task Executor` emits `TaskExecutionFailed`. The payload may include retry policy information or the orchestrator retrieves it.
2.  The `Workflow Orchestrator` consumes `TaskExecutionFailed`.
3.  It checks the task's retry policy (e.g., maximum attempts, backoff strategy).
4.  **If a retry is allowed:**
    *   The `Workflow Orchestrator` emits `TaskRetryScheduled`. This event indicates an intention to retry and might include details like the delay before the next attempt and the new attempt number.
    *   The `State Manager` records this.
    *   The `Task Dispatcher` might consume `TaskRetryScheduled` to manage the delay and then re-issue an `TaskExecute` command for the retry attempt.
5.  **If no more retries are allowed:** The `Workflow Orchestrator` proceeds with workflow-level failure handling. 
