# Flow: Task Execution Failure (Due to Agent Failure)

This diagram illustrates a task failing because an agent it invoked either returned an error, the execution itself failed (e.g., timeout), or a dispatch failure occurred.

```mermaid
sequenceDiagram
    participant WO as Workflow Orchestrator
    participant TD as Task Dispatcher
    participant TE as Task Executor
    participant AD as Agent Dispatcher
    participant AR as Agent Runtime
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

    Note over TE: Task logic requires an agent

    TE->>NATS: Publishes `TriggerAgent` (Command)
    NATS-->>AD: Delivers `TriggerAgent`
    activate AD

    alt Agent-Side Error (e.g., agent/02_execution_agent_error.md)
        AD->>NATS: Relays to Agent Runtime
        deactivate AD
        NATS-->>AR: Delivers command
        activate AR
        AR->>NATS: Publishes `AgentExecutionStarted` (optional)
        NATS-->>SM: Delivers `AgentExecutionStarted`
        AR-->>NATS: Publishes `AgentExecutionResult` (AGENT_ERROR) back to TE
        deactivate AR
        NATS-->>TE: Delivers `AgentExecutionResult` {status: "AGENT_ERROR", ...}
        TE->>NATS: Publishes `AgentExecutionResult` <br>Payload: {agent_execution_status: "AGENT_ERROR"}
        NATS-->>SM: Delivers `AgentExecutionResult`

    else Execution Timeout/Failure (e.g., agent/03_execution_timeout.md)
        deactivate AD # AD might still be active if relay fails or AR never responds
        Note over TE: Timeout waiting for AgentExecutionResult
        TE->>NATS: Publishes `AgentExecutionFailed` <br>Payload: {error: {message: "Timeout", ...}}
        NATS-->>SM: Delivers `AgentExecutionFailed`

    else Dispatch Failure (e.g., agent/04_dispatch_failure.md)
        Note over AD: Fails to dispatch to any Agent Runtime
        deactivate AD
        AD->>NATS: Publishes `AgentDispatchFailed`
        NATS-->>SM: Delivers `AgentDispatchFailed`
        Note over TE: Task likely times out here too, then emits AgentExecutionFailed (or TE gets direct error if AD interaction is sync)
        TE->>NATS: Publishes `AgentExecutionFailed` <br>Payload: {error: {message: "Dispatch related or timeout", ...}}
        NATS-->>SM: Delivers `AgentExecutionFailed`
    end
    

    Note over TE: Task logic handles the agent interaction failure and decides to fail the task

    TE->>NATS: Publishes `TaskExecutionFailed`<br>Payload: {error: {message: "Task failed due to agent error...", ...}}
    NATS-->>SM: Delivers `TaskExecutionFailed`
    SM-->>SM: Records TaskExecutionFailed
    NATS-->>WO: Delivers `TaskExecutionFailed`

    Note over WO: Processes task failure
```

This flow shows:
1.  Standard task scheduling and start.
2.  The `Task Executor` attempts to call an agent.
3.  **Agent Call Fails (Several scenarios shown):**
    *   **Agent-Side Error:** The `AgentRuntime` returns `AgentExecutionResult` with an `AGENT_ERROR` status. `TE` emits `AgentExecutionResult` (reflecting agent error).
    *   **Execution Timeout/Failure:** `TE` times out waiting for `AgentExecutionResult`, leading to `AgentExecutionFailed`.
    *   **Dispatch Failure:** `AgentDispatcher` fails to dispatch, emits `AgentDispatchFailed`. `TE` likely times out and also emits `AgentExecutionFailed`.
4.  The `Task Executor`'s logic determines the agent interaction failure is critical, and the task itself must fail.
5.  The `Task Executor` emits `TaskExecutionFailed`.
6.  `State Manager` and `Workflow Orchestrator` process the task failure. 
