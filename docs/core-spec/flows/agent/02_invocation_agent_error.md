# Flow: Agent Execution with Agent-Side Error (by Task Executor)

This diagram shows the sequence of events when a `task.Executor` invokes an agent, and the `agent.Runtime` executes the agent logic, but the agent's own logic encounters an error.

```mermaid
sequenceDiagram
    participant TE as Task Executor
    participant AD as Agent Dispatcher
    participant AR as Agent Runtime
    participant SM as State Manager
    participant NATS

    TE->>NATS: Publishes `TriggerAgent` (Command)<br>Subject: `compozy.commands.agent.<workflow_id>.trigger.<agent_id>`
    NATS-->>AD: Delivers `TriggerAgent`

    AD->>NATS: Relays to specific Agent Runtime instance
    NATS-->>AR: Delivers command to Agent Runtime

    AR->>NATS: Publishes `AgentExecutionStarted` (State Event)<br>Subject: `compozy.events.agent.instance.<agent_exec_id>.started`
    NATS-->>SM: Delivers `AgentExecutionStarted`
    SM-->>SM: Records AgentExecutionStarted

    Note over AR: Agent logic encounters an internal error

    AR->>NATS: Publishes `AgentExecutionResult` (Command Result)<br>Subject: (reply_to_subject from TriggerAgent)<br>Payload: {status: "AGENT_ERROR", error: {message, code, details}}
    NATS-->>TE: Delivers `AgentExecutionResult`

    Note over TE: Processes agent result indicating an agent error

    TE->>NATS: Publishes `AgentExecutionResult` (State Event)<br>Subject: `compozy.events.agent.instance.<agent_exec_id>.completed`<br>Payload: {agent_execution_status: "AGENT_ERROR"}
    NATS-->>SM: Delivers `AgentExecutionResult`
    SM-->>SM: Records AgentExecutionResult (noting agent_execution_status)
```

This flow involves:
1.  The `Task Executor` sending a `TriggerAgent` command.
2.  The `Agent Dispatcher` forwarding it to an `Agent Runtime`.
3.  The `Agent Runtime` emitting `AgentExecutionStarted`.
4.  The `Agent Runtime`'s internal agent logic failing, leading it to return an `AgentExecutionResult` with an "AGENT_ERROR" status and error details.
5.  The `Task Executor` receiving this error result and emitting `AgentExecutionResult`, reflecting the `agent_execution_status` as "AGENT_ERROR". The execution itself (request-reply) completed, but the agent's work did not succeed.
6.  The `State Manager` consumes the state events for tracking. 
