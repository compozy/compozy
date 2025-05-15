# Flow: Workflow Waiting and Resume

This diagram illustrates a workflow execution pausing to wait for an external signal or condition, and then being resumed.

```mermaid
sequenceDiagram
    participant Client as Client/API/ExternalEventSource
    participant WFEngine as Workflow Engine (Orchestrator)
    participant SM as State Manager
    participant NATS

    Note over WFEngine: Workflow logic determines it needs to pause (e.g., wait for an external event, a specific time, or manual intervention)
    WFEngine->>NATS: Publishes `WorkflowExecutionWaitingStarted` (State Event)<br>Subject: `compozy.events.workflow.instance.<workflow_exec_id>.waiting_started`<br>Payload: {wait_reason: "Waiting for external payment confirmation", ...}
    NATS-->>SM: Delivers `WorkflowExecutionWaitingStarted`
    SM-->>SM: Records WorkflowExecutionWaitingStarted

    Note over WFEngine: Workflow is now in a WAITING state. It may release some active resources.

    ...Some time later...

    Client->>NATS: Publishes `WorkflowResume` (Command)<br>Subject: `compozy.commands.workflow.resume.<workflow_exec_id>`<br>Payload: {workflow_exec_id, resume_data: {...}}
    NATS-->>WFEngine: Delivers `WorkflowResume`

    Note over WFEngine: Processes resume command for <workflow_exec_id>
    WFEngine->>NATS: Publishes `WorkflowExecutionWaitingEnded` (State Event)<br>Subject: `compozy.events.workflow.instance.<workflow_exec_id>.waiting_ended`<br>Payload: {resume_data}
    NATS-->>SM: Delivers `WorkflowExecutionWaitingEnded`
    SM-->>SM: Records WorkflowExecutionWaitingEnded
    
    WFEngine->>NATS: Publishes `WorkflowExecutionStarted` (or a more specific `WorkflowExecutionResumed`) (State Event)<br>Subject: `compozy.events.workflow.instance.<workflow_exec_id>.started` (or `.resumed`)<br>Payload: {details_of_resumption_point}
    NATS-->>SM: Delivers event
    SM-->>SM: Records event

    Note over WFEngine: Workflow logic continues from where it left off, potentially using `resume_data`.
    Note over WFEngine: Proceeds to schedule next tasks, etc., eventually leading to WorkflowExecutionSuccess or Failed.

```

This flow involves:
1.  The `Workflow Engine` (WFEngine), during execution, determines the workflow needs to pause (e.g., for an external event, a human approval, a scheduled delay).
2.  `WFEngine` emits `WorkflowExecutionWaitingStarted`.
3.  The `State Manager` records this state. The workflow is now effectively paused at that point.
4.  Later, a `Client` (or an external system, or a timer) issues a `WorkflowResume` command, targeting the paused `workflow_exec_id` and potentially providing data needed for resumption.
5.  `WFEngine` receives the `WorkflowResume` command.
6.  `WFEngine` emits `WorkflowExecutionWaitingEnded` to signify the end of the waiting period.
7.  `WFEngine` then typically emits `WorkflowExecutionStarted` (or a more specific `WorkflowExecutionResumed` event if defined) to indicate the workflow is actively processing again.
8.  The workflow continues its execution from the point it was paused, potentially using information from the `resume_data`.

</rewritten_file> 
