# Flow: Agent Execution Failure (e.g., Timeout by Task Executor)

This diagram shows the sequence of events when a `task.Executor` invokes an agent, but the execution fails, for example, due to a NATS timeout waiting for the `AgentExecutionResult`.

```mermaid
sequenceDiagram
    participant TE as Task Executor
    participant AD as Agent Dispatcher
    participant AR as Agent Runtime
    participant SM as State Manager
    participant NATS

    TE->>NATS: Publishes `TriggerAgent` (Command)<br>Subject: `compozy.commands.agent.<workflow_id>.trigger.<agent_id>`
    NATS-->>AD: Delivers `TriggerAgent`

    Note over AD: Attempts to dispatch to an Agent Runtime
    AD->>NATS: Relays to specific Agent Runtime instance (or attempts to)
    NATS-->>AR: Delivers command (or message is lost / AR is down / no AR available)

    Note over TE: Waits for `AgentExecutionResult` on reply_to_subject...
    Note over TE: Timeout occurs before receiving a response

    TE->>NATS: Publishes `AgentExecutionFailed` (State Event)<br>Subject: `compozy.events.agent.instance.<agent_exec_id>.failed`<br>Payload: {error: {message: "NATS request timed out waiting for AgentExecutionResult", code: "AGENT_INVOCATION_TIMEOUT"}}
    NATS-->>SM: Delivers `AgentExecutionFailed`
    SM-->>SM: Records AgentExecutionFailed
```

This flow involves:
1.  The `Task Executor` sending a `TriggerAgent` command.
2.  The `Agent Dispatcher` attempts to forward it.
3.  The `Task Executor` does not receive an `AgentExecutionResult` within its configured timeout period. This could be due to the `Agent Runtime` being down, network issues, or the `Agent Dispatcher` failing to find a runtime.
4.  The `Task Executor` then emits `AgentExecutionFailed`.
5.  The `State Manager` consumes the `AgentExecutionFailed` event.

*Note: An `AgentExecutionStarted` might or might not have been emitted by an `Agent Runtime` depending on where the failure occurred. This diagram focuses on the timeout from the `Task Executor`'s perspective.* 
