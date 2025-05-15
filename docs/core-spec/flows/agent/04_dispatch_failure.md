# Flow: Agent Dispatch Failure

This diagram shows the sequence of events when the `agent.Dispatcher` fails to dispatch a `TriggerAgent` command to an `agent.Runtime`. This might happen if no compatible runtime is registered or available.

```mermaid
sequenceDiagram
    participant TE as Task Executor
    participant AD as Agent Dispatcher
    participant SM as State Manager
    participant NATS

    TE->>NATS: Publishes `TriggerAgent` (Command)<br>Subject: `compozy.commands.agent.<workflow_id>.trigger.<agent_id>`
    NATS-->>AD: Delivers `TriggerAgent`

    Note over AD: Attempts to find a suitable Agent Runtime for <agent_id>
    Note over AD: Fails to find/dispatch to an Agent Runtime

    AD->>NATS: Publishes `AgentDispatchFailed` (State Event)<br>Subject: `compozy.events.agent.instance.<agent_exec_id_or_request_id>.dispatch_failed` (Note: identifier might be from original request if exec_id not yet created)
    NATS-->>SM: Delivers `AgentDispatchFailed`
    SM-->>SM: Records AgentDispatchFailed

    Note over TE: May timeout waiting for AgentExecutionResult, leading to AgentExecutionFailed from TE later if no direct error is sent back to TE from AD.
    Note over TE: Alternatively, AD might send a direct error reply to TE's request if using request-reply for TriggerAgent to AD.
```

This flow involves:
1.  The `Task Executor` sending a `TriggerAgent` command, targeting a specific agent ID.
2.  The `Agent Dispatcher` receives the command.
3.  The `Agent Dispatcher` is unable to find a registered and available `Agent Runtime` capable of handling the specified agent ID, or fails in its attempt to communicate with a selected runtime.
4.  The `Agent Dispatcher` emits an `AgentDispatchFailed` event.
5.  The `State Manager` consumes this event.
6.  The original `Task Executor` might subsequently time out and emit an `AgentExecutionFailed` if the `TriggerAgent` command to the `Agent Dispatcher` was a fire-and-forget to the dispatcher, and it was expecting a result on a `reply_to_subject`. If `TriggerAgent` to `AD` is synchronous, `AD` could return an error directly to `TE`.

*Clarification: The exact mechanism for how the `Task Executor` becomes aware of the dispatch failure (e.g., direct error reply from Dispatcher vs. timeout) depends on the NATS communication pattern between `TE` and `AD` for the `TriggerAgent` command.* 
