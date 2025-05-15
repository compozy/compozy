# Flow: Successful Task Execution with Agent Call

This diagram illustrates a task that, as part of its execution, successfully calls an agent.

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

    TE->>NATS: Publishes `TriggerAgent` (Command) to Agent Dispatcher
    NATS-->>AD: Delivers `TriggerAgent`
    activate AD

    AD->>NATS: Relays to specific Agent Runtime instance
    deactivate AD
    NATS-->>AR: Delivers command to Agent Runtime
    activate AR

    AR->>NATS: Publishes `AgentExecutionStarted`
    NATS-->>SM: Delivers `AgentExecutionStarted`
    SM-->>SM: Records AgentExecutionStarted

    AR-->>NATS: Publishes `AgentExecutionResult` (SUCCESS) back to TE
    deactivate AR
    NATS-->>TE: Delivers `AgentExecutionResult`
    
    TE->>NATS: Publishes `AgentExecutionResult`
    NATS-->>SM: Delivers `AgentExecutionResult`
    SM-->>SM: Records AgentExecutionResult

    Note over TE: Task logic continues with agent output, then completes

    TE->>NATS: Publishes `TaskExecutionResult`
    NATS-->>SM: Delivers `TaskExecutionResult`
    SM-->>SM: Records TaskExecutionResult
    NATS-->>WO: Delivers `TaskExecutionResult`

    Note over WO: Processes task completion
```

This flow extends the basic task execution by adding an agent call:
1.  Initial steps: `TaskScheduled`, `TaskExecute`, `TaskExecutionStarted`.
2.  The `Task Executor` determines it needs to call an agent.
3.  **Agent Call Sub-flow (simplified, see `agent/01_successful_execution.md` for full detail):**
    *   `TE` sends `TriggerAgent` to `AgentDispatcher`.
    *   `AgentDispatcher` relays to `AgentRuntime`.
    *   `AgentRuntime` emits `AgentExecutionStarted`.
    *   `AgentRuntime` returns `AgentExecutionResult` (success) to `TE`.
    *   `TE` emits `AgentExecutionResult`.
4.  The `Task Executor` uses the agent's output and completes its main logic.
5.  The `Task Executor` emits `TaskExecutionResult`.
6.  The `State Manager` and `Workflow Orchestrator` consume events. 
