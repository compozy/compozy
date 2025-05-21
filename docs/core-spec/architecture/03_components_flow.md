# Component Interactions

This document provides a visual representation of how components in the Compozy Workflow Engine interact through NATS events, including **Commands**, **State Events**, and **Log Events**. The sequence diagram below illustrates a typical workflow execution, from triggering a workflow to executing tasks and agents/tools, with state persistence, monitoring, and task dispatch notifications.

## Sequence Diagram

The following Mermaid sequence diagram shows the interactions among components, focusing on a scenario where a workflow is triggered, a task is executed, an agent or tool is invoked, and the system handles success or failure outcomes. The `nats.Client` and `nats.Server` facilitate communication but are not shown as active participants in the sequence for simplicity, as they handle the underlying message transport.

```mermaid
sequenceDiagram
    participant API as api.Service
    participant ORCH as system.Orchestrator
    participant WFEXEC as workflow.Executor
    participant TASKEXEC as task.Executor
    participant RUNTIME as system.Runtime
    participant STATE as state.Manager
    participant MONITOR as system.Monitoring
    participant NATS as "nats.Client/Server"

    Note over API,MONITOR: Workflow Trigger and Execution
    API->>ORCH: WorkflowTrigger (sync)
    ORCH->>STATE: WorkflowExecutionStarted
    ORCH->>MONITOR: WorkflowExecutionStarted
    ORCH->>API: Acknowledge (workflow_exec_id)
    ORCH->>WFEXEC: WorkflowExecute (async)
    
    Note over ORCH,TASKEXEC: Task Dispatch and Execution
    WFEXEC->>ORCH: Request Task Execution
    ORCH->>STATE: TaskDispatched
    ORCH->>MONITOR: TaskDispatched
    ORCH->>STATE: TaskExecutionStarted
    ORCH->>MONITOR: TaskExecutionStarted
    ORCH->>TASKEXEC: TaskExecute (sync)

    Note over TASKEXEC,RUNTIME: Task Invokes Agent/Tool
    TASKEXEC->>RUNTIME: AgentExecute (sync)
    RUNTIME->>STATE: AgentExecutionStarted
    RUNTIME->>MONITOR: AgentExecutionStarted
    alt Agent Success
        RUNTIME->>STATE: AgentExecutionSuccess
        RUNTIME->>MONITOR: AgentExecutionSuccess
        RUNTIME->>TASKEXEC: Agent Result
    else Agent Failure
        RUNTIME->>STATE: AgentExecutionFailed
        RUNTIME->>MONITOR: AgentExecutionFailed
        RUNTIME->>ORCH: AgentExecutionFailed
        RUNTIME->>TASKEXEC: Agent Error
    end

    Note over TASKEXEC,WFEXEC: Task Completion
    alt Task Success
        TASKEXEC->>STATE: TaskExecutionSuccess
        TASKEXEC->>MONITOR: TaskExecutionSuccess
        TASKEXEC->>WFEXEC: Task Result
    else Task Failure
        TASKEXEC->>STATE: TaskExecutionFailed
        TASKEXEC->>MONITOR: TaskExecutionFailed
        TASKEXEC->>STATE: TaskRetryScheduled
        TASKEXEC->>MONITOR: TaskRetryScheduled
        TASKEXEC->>WFEXEC: Task Error
    end

    Note over WFEXEC,MONITOR: Workflow Completion or Cancellation
    alt Workflow Success
        WFEXEC->>STATE: WorkflowExecutionSuccess
        WFEXEC->>MONITOR: WorkflowExecutionSuccess
    else Workflow Failure
        WFEXEC->>STATE: WorkflowExecutionFailed
        WFEXEC->>MONITOR: WorkflowExecutionFailed
    else Workflow Timeout
        WFEXEC->>STATE: WorkflowExecutionTimedOut
        WFEXEC->>MONITOR: WorkflowExecutionTimedOut
    else Workflow Cancelled
        API->>ORCH: WorkflowCancel (async)
        ORCH->>WFEXEC: (internal signal)
        WFEXEC->>STATE: WorkflowExecutionCancelled
        WFEXEC->>MONITOR: WorkflowExecutionCancelled
    end

    Note over API,MONITOR: Logging
    API->>MONITOR: LogEmitted (async)
    ORCH->>MONITOR: LogEmitted (async)
    WFEXEC->>MONITOR: LogEmitted (async)
    TASKEXEC->>MONITOR: LogEmitted (async)
    RUNTIME->>MONITOR: LogEmitted (async)
    STATE->>MONITOR: LogEmitted (async)
    MONITOR->>MONITOR: LogEmitted (async)
    ORCH->>ORCH: LogEmitted (self-consume, async)

    Note over API,MONITOR: Workflow Execution Complete
    Note over NATS: NATS Client/Server handles all message transport
```
