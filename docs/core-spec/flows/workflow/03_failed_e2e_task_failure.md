# Flow: Workflow Execution Failure (Due to Task Failure)

This diagram shows a workflow failing because one of its constituent tasks fails.

```mermaid
sequenceDiagram
    participant Client as Client/API/Trigger
    participant WFEngine as Workflow Engine (Orchestrator)
    participant TaskExecPath as Task Execution Path
    participant SM as State Manager
    participant NATS

    Client->>NATS: `WorkflowExecute`
    NATS-->>WFEngine: Delivers `WorkflowExecute`

    WFEngine->>NATS: `WorkflowExecutionStarted`
    NATS-->>SM: Records `WorkflowExecutionStarted`

    Note over WFEngine: Initiates a task
    WFEngine->>TaskExecPath: Initiates Task (e.g., see `../task/04_execution_failure_internal.md` or `../task/05_execution_failure_due_to_tool.md`)
    activate TaskExecPath
    Note over TaskExecPath: Task Execution starts... and then fails.
    TaskExecPath-->>WFEngine: Task `TaskExecutionFailed` event
    deactivate TaskExecPath

    Note over WFEngine: Processes task failure. Determines workflow should fail (e.g., no retry, or final retry failed).
    WFEngine->>NATS: Publishes `WorkflowExecutionFailed` (State Event)<br>Subject: `compozy.events.workflow.instance.<workflow_exec_id>.failed`<br>Payload: {error: {message: "Workflow failed due to task X failure", details: {failed_task_id, failed_task_exec_id, task_error_details}}}
    NATS-->>SM: Delivers `WorkflowExecutionFailed`
    SM-->>SM: Records WorkflowExecutionFailed
    NATS-->>Client: (Optional) Notify Client of workflow failure

```

This flow involves:
1.  Standard workflow initiation: `WorkflowExecute`, `WorkflowExecutionStarted`.
2.  The `Workflow Engine` (WFEngine) initiates a task.
3.  **Task Failure:** The task execution path results in a `TaskExecutionFailed` event being sent to the `WFEngine`. (The reason for the task failure could be internal, or due to a failed tool/agent call, as detailed in various task failure flows like `../task/04_execution_failure_internal.md`, `../task/05_execution_failure_due_to_tool.md`, or `../task/06_execution_failure_due_to_agent.md`).
4.  The `WFEngine` processes the `TaskExecutionFailed` event. Based on the workflow definition and error handling policies (e.g., no retries left for the task, or the failure is critical), it decides to fail the entire workflow.
5.  The `WFEngine` emits `WorkflowExecutionFailed`, including details about the source of the failure.
6.  The `State Manager` records the workflow failure.
7.  Optionally, the `Client` is notified. 
