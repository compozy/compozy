# Subjects Definitions

This document provides a glossary of all subjects and events used in the Compozy system.

## Conventions

Compozy uses distinct subject naming patterns that correspond to message types (State Events, Commands, Log Events).

#### 1. State Event Subjects
Used for events that record state changes for event sourcing.

Pattern: `compozy.<correlation_id>.<entity>.evts.<identifier>.<event_type>`

Where:
-   `<entity>`: "workflow", "task", "agent", "tool"
-   `<identifier>`: ID of the entity instance (e.g., `workflow_exec_id`, `task_exec_id`, `agent_exec_id`, `tool_exec_id`)
-   `<event_type>`: Specific event name (e.g., "started", "success", "failed", "waiting_started")

#### 2. Command Subjects
Used for messages that instruct components to perform actions.

Pattern: `compozy.<correlation_id>.<entity>.cmds.<identifier>.<action>`

Where:
-   `<entity>`: Target entity type ("workflow", "task", "agent", "tool")
-   `<identifier>`: Target entity identifier (e.g., `workflow_id`, `workflow_exec_id`, `task_id`, `task_exec_id`, `agent_id`, `tool_id`)
    *Note: The specific identifier depends on the command's target.*
-   `<action>`: Action to perform (e.g., "execute", "cancel", "resume", "trigger")

#### 3. Log Event Subjects
Used for emitting log messages from various system components.

Pattern: `compozy.logs.<correlation_id>.<component>.<log_level>`

Where:
-   `<correlation_id>`: Identifier to correlate messages, often a workflow/task execution ID or a unique operation ID.
-   `<component>`: Sanitized, lowercase, underscore-separated name of the component emitting the log (e.g., `workflow_orchestrator`, `task_executor`, `api_service`). This should correspond to names listed in `docs/core-spec/architecture/02_components.md`.
-   `<log_level>`: The severity level of the log (`debug`, `info`, `warn`, `error`).

---

## Workflow Events

### Commands
-   [WorkflowTrigger](workflow/cmds/trigger.md#command-triggerworkflow) 
    - Subject: `compozy.<correlation_id>.workflow.cmds.<workflow_id>.trigger`
-   [WorkflowExecute](workflow/cmds/execute.md#command-executeworkflow) 
    - Subject: `compozy.<correlation_id>.workflow.cmds.<workflow_exec_id>.execute`
-   [WorkflowPause](workflow/cmds/pause.md#command-pauseworkflow) 
    - Subject: `compozy.<correlation_id>.workflow.cmds.<workflow_exec_id>.pause`
-   [WorkflowResume](workflow/cmds/resume.md#command-resumeworkflow) 
    - Subject: `compozy.<correlation_id>.workflow.cmds.<workflow_exec_id>.resume`
-   [WorkflowCancel](workflow/cmds/cancel.md#command-cancelworkflow) 
    - Subject: `compozy.<correlation_id>.workflow.cmds.<workflow_exec_id>.cancel`

### State Events
-   [WorkflowExecutionStarted](workflow/state/execution_started.md#state-event-workflowexecutionstarted)
    - Subject: `compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.started`
-   [WorkflowExecutionPaused](workflow/state/execution_paused.md#state-event-workflowexecutionpaused)
    - Subject: `compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.paused`
-   [WorkflowExecutionResumed](workflow/state/execution_resumed.md#state-event-workflowexecutionresumed)
    - Subject: `compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.resumed`
-   [WorkflowExecutionSuccess](workflow/state/execution_success.md#state-event-workflowexecutioncompleted)
    - Subject: `compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.success`
-   [WorkflowExecutionFailed](workflow/state/execution_failed.md#state-event-workflowexecutionfailed)
    - Subject: `compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.failed`
-   [WorkflowExecutionCanceled](workflow/state/execution_canceled.md#state-event-workflowexecutioncanceled)
    - Subject: `compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.canceled`
-   [WorkflowExecutionTimedOut](workflow/state/execution_timed_out.md#state-event-workflowexecutiontimedout) 
    - Subject: `compozy.<correlation_id>.workflow.evts.<workflow_exec_id>.timed_out`

---

## Task Events

### Commands
-   [TaskTrigger](task/cmds/trigger.md#command-triggerspecifictask) 
    - Subject: `compozy.<correlation_id>.task.cmds.<task_id>.trigger`
-   [TaskExecute](task/cmds/execute.md#command-executetask) 
    - Subject: `compozy.<correlation_id>.task.cmds.<task_exec_id>.execute`
-   [TaskResume](task/cmds/resume.md#command-resumewaitingtask) 
    - Subject: `compozy.<correlation_id>.task.cmds.<task_exec_id>.resume`

### State Events
-   [TaskDispatched](task/state/dispatched.md#state-event-taskdispatched)
    - Subject: `compozy.<correlation_id>.task.evts.<task_exec_id>.dispatched`
-   [TaskExecutionStarted](task/state/execution_started.md#state-event-taskexecutionstarted)
    - Subject: `compozy.<correlation_id>.task.evts.<task_exec_id>.started`
-   [TaskWaitingStarted](task/state/waiting_started.md#state-event-waitingstarted)
    - Subject: `compozy.<correlation_id>.task.evts.<task_exec_id>.waiting_started`
-   [TaskWaitingEnd](task/state/waiting_ended.md#state-event-waitingended)
    - Subject: `compozy.<correlation_id>.task.evts.<task_exec_id>.waiting_ended`
-   [TaskWaitingTimedOut](task/state/waiting_timed_out.md#state-event-waitingtimedout)
    - Subject: `compozy.<correlation_id>.task.evts.<task_exec_id>.waiting_timed_out`
-   [TaskExecutionSuccess](task/state/execution_success.md#state-event-taskexecutioncompleted)
    - Subject: `compozy.<correlation_id>.task.evts.<task_exec_id>.success`
-   [TaskExecutionFailed](task/state/execution_failed.md#state-event-taskexecutionfailed)
    - Subject: `compozy.<correlation_id>.task.evts.<task_exec_id>.failed`

---

## Agent Events

### Commands
-   [AgentExecute](agent/cmds/execute.md#command-executeagent) 
    - Subject: `compozy.<correlation_id>.agent.cmds.<agent_id>.execute`

### State Events
-   [AgentExecutionStarted](agent/state/execution_started.md#state-event-agentexecutionstarted)
    - Subject: `compozy.<correlation_id>.agent.evts.<agent_exec_id>.started`
-   [AgentExecutionSuccess](agent/state/execution_success.md#state-event-agentexecutioncompleted)
    - Subject: `compozy.<correlation_id>.agent.evts.<agent_exec_id>.success`
-   [AgentExecutionFailed](agent/state/execution_failed.md#state-event-agentexecutionfailed)
    - Subject: `compozy.<correlation_id>.agent.evts.<agent_exec_id>.failed`

---

## Tool Events

### Commands
-   [ToolExecute](tool/cmds/execute.md#command-executetool) 
    - Subject: `compozy.<correlation_id>.tool.cmds.<tool_id>.execute`

### State Events
-   [ToolExecutionStarted](tool/state/execution_started.md#state-event-toolexecutionstarted)
    - Subject: `compozy.<correlation_id>.tool.evts.<tool_exec_id>.started`
-   [ToolExecutionSuccess](tool/state/execution_success.md#state-event-toolexecutioncompleted)
    - Subject: `compozy.<correlation_id>.tool.evts.<tool_exec_id>.success`
-   [ToolExecutionFailed](tool/state/execution_failed.md#state-event-toolexecutionfailed)
    - Subject: `compozy.<correlation_id>.tool.evts.<tool_exec_id>.failed`

---

## Log Events

-   [LogEmitted](log/events/emitted.md#log-event-logemitted)
    - Subject: `compozy.<correlation_id>.<component>.logs.<component_id>.<log_level>`
