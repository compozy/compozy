# Flow: Successful Agent Execution (by Task Executor)

This diagram shows the sequence of events when a `task.Executor` successfully invokes an agent. The `agent.Dispatcher` routes the request to an available `agent.Runtime`, which executes the agent logic and returns a successful result.

```mermaid
sequenceDiagram
    participant TE as Task Executor
    participant AD as Agent Dispatcher
    participant AR as Agent Runtime
    participant SM as State Manager
    participant NATS

    TE->>NATS: Publishes `TriggerAgent` (Command)<br>Subject: `compozy.commands.agent.<workflow_id>.trigger.<agent_id>`<br>Payload: {agent_id, input_args, reply_to_subject}
    NATS-->>AD: Delivers `TriggerAgent`

    Note over AD: Identifies available Agent Runtime
    AD->>NATS: Relays/Publishes `TriggerAgent` (or similar internal command)<br>to specific Agent Runtime instance topic
    NATS-->>AR: Delivers `TriggerAgent` command to selected Agent Runtime

    Note over AR: Starts agent execution logic

    AR->>NATS: Publishes `AgentExecutionStarted` (State Event)<br>Subject: `compozy.events.agent.instance.<agent_exec_id>.started`
    NATS-->>SM: Delivers `AgentExecutionStarted`
    SM-->>SM: Records AgentExecutionStarted

    Note over AR: Agent logic completes successfully

    AR->>NATS: Publishes `AgentExecutionResult` (Command Result)<br>Subject: (reply_to_subject from TriggerAgent, e.g., `compozy.results.agent.<agent_exec_id>`)<br>Payload: {status: "SUCCESS", output}
    NATS-->>TE: Delivers `AgentExecutionResult`

    Note over TE: Processes successful agent output

    TE->>NATS: Publishes `AgentExecutionResult` (State Event)<br>Subject: `compozy.events.agent.instance.<agent_exec_id>.completed`<br>Payload: {agent_execution_status: "SUCCESS"}
    NATS-->>SM: Delivers `AgentExecutionResult`
    SM-->>SM: Records AgentExecutionResult
```

This flow involves:
1.  The `Task Executor` sending a `TriggerAgent` command.
2.  The `Agent Dispatcher` receiving the command and forwarding it to an available `Agent Runtime`.
3.  The `Agent Runtime` acknowledging the start of execution by emitting `AgentExecutionStarted`.
4.  The `Agent Runtime` successfully executing the agent logic and returning the `AgentExecutionResult` with a "SUCCESS" status and output.
5.  The `Task Executor`, after processing the result, emitting `AgentExecutionResult`.
6.  The `State Manager` consumes the state events for tracking. 
