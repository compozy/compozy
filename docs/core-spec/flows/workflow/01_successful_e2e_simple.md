# Flow: Successful Workflow Execution (Simple E2E)

This diagram illustrates the end-to-end successful execution of a simple workflow, for example, one that contains a single task which completes successfully without external calls.

```mermaid
sequenceDiagram
    participant Client as Client/API/Trigger
    participant WFEngine as Workflow Engine (Orchestrator)
    participant TaskExecPath as Task Execution Path (see task/01*)
    participant SM as State Manager
    participant NATS

    Client->>NATS: Publishes `WorkflowExecute` (Command)<br>Subject: `compozy.commands.workflow.execute.<workflow_id>`<br>Payload: {workflow_id, inputs, ...}
    NATS-->>WFEngine: Delivers `WorkflowExecute`

    Note over WFEngine: Initializes workflow execution
    WFEngine->>NATS: Publishes `WorkflowExecutionStarted` (State Event)<br>Subject: `compozy.events.workflow.instance.<workflow_exec_id>.started`<br>Payload: {workflow_id, workflow_exec_id, inputs, ...}
    NATS-->>SM: Delivers `WorkflowExecutionStarted`
    SM-->>SM: Records WorkflowExecutionStarted

    Note over WFEngine: Identifies first task(s) to execute
    WFEngine->>TaskExecPath: Initiates Task Execution (e.g., `TaskScheduled` leading to flow in `task/01_successful_execution_no_callout.md`)
    activate TaskExecPath
    Note over TaskExecPath: Task Scheduled, Started, Completed
    TaskExecPath-->>WFEngine: Task(s) Completed (e.g., via `TaskExecutionResult` event)
    deactivate TaskExecPath

    Note over WFEngine: All tasks completed, workflow logic determines completion
    WFEngine->>NATS: Publishes `WorkflowExecutionSuccess` (State Event)<br>Subject: `compozy.events.workflow.instance.<workflow_exec_id>.completed`<br>Payload: {outputs, ...}
    NATS-->>SM: Delivers `WorkflowExecutionSuccess`
    SM-->>SM: Records WorkflowExecutionSuccess
    NATS-->>Client: (Optional) Delivers `WorkflowExecutionSuccess` or specific result notification

```

This flow involves:
1.  A `Client` (or an API call, or an external trigger) initiates a workflow by sending an `WorkflowExecute` command.
2.  The `Workflow Engine` (specifically its orchestrator component) receives the command.
3.  The `WFEngine` emits `WorkflowExecutionStarted`.
4.  The `WFEngine` orchestrates the execution of the workflow's tasks. In this simple case, it involves a single task execution that completes successfully (this part is abstracted and detailed in task flow diagrams like `../task/01_successful_execution_no_callout.md`).
5.  Upon successful completion of all necessary tasks and fulfillment of workflow logic, the `WFEngine` emits `WorkflowExecutionSuccess`.
6.  The `State Manager` records all relevant state events.
7.  Optionally, the `Client` might be notified of the workflow completion, either by subscribing to `WorkflowExecutionSuccess` or through a dedicated result message. 
