# Component Naming Standardization

This document outlines the standardized architectural component names within the Compozy Workflow Engine project. These names align with widely recognized terminology in distributed systems and workflow management, enhancing clarity and maintainability.

## Component Names

1. **API Service**
   - **Package:** `api`
   - **Struct:** `Service`
   - **Responsibilities:**
     - Exposes external APIs (REST) for managing and interacting with the workflow engine.
     - Handles endpoints for creating, updating, and deleting workflow definitions.
     - Facilitates triggering new workflow and task instances (e.g., `WorkflowTrigger`, `TaskTrigger`) and querying their status and history.
     - Supports manual interventions like resuming (`WorkflowResume`, `TaskResume`), pausing (`WorkflowPause`), or canceling (`WorkflowCancel`) workflows and tasks.
   - **Consumed Events:**
     - None (API Service primarily initiates commands via external requests).
   - **Produced Events:**
     - **Commands:**
       - `WorkflowTrigger` 
       - `WorkflowTrigger` 
       - `WorkflowPause` 
       - `WorkflowResume` 
       - `WorkflowCancel` 
       - `TaskTrigger` 
       - `TaskTrigger` 
       - `TaskResume` 
       - `TaskCancel` 
     - **Log Events:**
       - `LogEmitted` 

2. **System Orchestrator**
   - **Package:** `system`
   - **Struct:** `Orchestrator`
   - **Responsibilities:**
     - Acts as the central brain of the engine, initiating workflow executions based on triggers (e.g., `WorkflowTrigger`, `WorkflowTrigger`).
     - Interprets workflow definitions to manage sequence, conditional logic, and parallelism.
     - Initializes and maintains the complete state of all workflow and task instances.
     - Assigns unique execution IDs to all workflows and tasks.
     - Coordinates with other components to process control commands (e.g., `WorkflowPause`, `WorkflowCancel`, `WorkflowResume`), initiating the appropriate cascading commands when needed.
     - Handles task dispatching by producing `TaskDispatched` events for visibility and coordination.
   - **Consumed Events:**
     - **Commands:**
       - `WorkflowTrigger` 
       - `WorkflowPause` 
       - `WorkflowResume` 
       - `WorkflowCancel` 
       - `TaskTrigger` 
     - **State Events:**
       - `AgentExecutionFailed` 
     - **Log Events:**
       - `LogEmitted` 
   - **Produced Events:**
     - **Commands:**
       - `WorkflowExecute` 
       - `TaskExecute`
       - `WorkflowCancel`
       - `WorkflowPause`
       - `WorkflowResume`
     - **State Events:**
       - `WorkflowExecutionStarted`
       - `TaskExecutionStarted`
       - `TaskDispatched`
     - **Log Events:**
       - `LogEmitted` 

3. **State Manager**
   - **Package:** `state`
   - **Struct:** `Manager`
   - **Responsibilities:**
     - Manages persistence and retrieval of all workflow-related state data.
     - Implements event sourcing by recording state changes as immutable events in NATS JetStream (e.g., `WorkflowExecutionStarted`, `TaskExecutionFailed`).
     - Takes periodic snapshots of workflow instances for efficient state reconstruction.
     - Provides APIs for recording events and querying current or historical states.
   - **Consumed Events:**
     - **State Events:**
       - `WorkflowExecutionStarted` 
       - `WorkflowExecutionPaused` 
       - `WorkflowExecutionResumed` 
       - `WorkflowExecutionSuccess` 
       - `WorkflowExecutionFailed` 
       - `WorkflowExecutionCanceled` 
       - `WorkflowExecutionTimedOut` 
       - `TaskExecutionStarted` 
       - `TaskExecutionSuccess` 
       - `TaskExecutionFailed`
       - `TaskExecutionCanceled`
       - `TaskWaitingStarted` 
       - `TaskWaitingEnded` 
       - `TaskWaitingTimedOut` 
       - `TaskRetryScheduled` 
       - `TaskDispatched` 
       - `AgentExecutionStarted` 
       - `AgentExecutionSuccess` 
       - `AgentExecutionFailed`
       - `AgentExecutionCanceled`
       - `ToolExecutionStarted` 
       - `ToolExecutionSuccess` 
       - `ToolExecutionFailed`
       - `ToolExecutionCanceled`
   - **Produced Events:**
     - **Log Events:**
       - `LogEmitted` 

4. **Workflow Executor**
   - **Package:** `workflow`
   - **Struct:** `Executor`
   - **Responsibilities:**
     - Validates the workflow definition and its input parameters.
     - Executes the workflow logic.
     - Updates the workflow state, producing state update events (e.g., `WorkflowExecutionPaused`, `WorkflowExecutionSuccess`, `WorkflowExecutionFailed`, `WorkflowExecutionTimedOut`, `WorkflowExecutionResumed`).
     - Handles workflow cancellation, producing `WorkflowExecutionCanceled` event and propagating cancellation to all running child tasks by issuing `TaskCancel` commands.
     - Requests task execution from the orchestrator.
   - **Consumed Events:**
     - **Commands:**
       - `WorkflowExecute` 
       - `WorkflowCancel`
       - `WorkflowPause`
       - `WorkflowResume`
   - **Produced Events:**
     - **Commands:**
       - `TaskCancel`
     - **State Events:**
       - `WorkflowExecutionPaused` 
       - `WorkflowExecutionResumed` 
       - `WorkflowExecutionSuccess` 
       - `WorkflowExecutionFailed` 
       - `WorkflowExecutionCanceled` 
       - `WorkflowExecutionTimedOut` 
     - **Log Events:**
       - `LogEmitted` 

5. **Task Executor**
   - **Package:** `task`
   - **Struct:** `Executor`
   - **Responsibilities:**
     - Validates the task definition and its input parameters.
     - Operates as a pool of workers subscribing to NATS task queues (e.g., `TaskExecute`, `TaskResume`).
     - Executes task logic, including Go-native code or invoking tools/agents via NATS to `system.Runtime`.
     - Handles task cancellation by stopping local execution and propagating cancellation to any running agents or tools.
     - Updates task state, producing state update events (e.g., `TaskExecutionSuccess`, `TaskExecutionFailed`, `TaskWaitingStarted`, `TaskWaitingEnded`, `TaskWaitingTimedOut`).
     - Schedules retries for failed tasks, producing `TaskRetryScheduled` based on retry policies.
     - Reports task outcomes (success, failure, output) back to the workflow executor.
   - **Consumed Events:**
     - **Commands:**
       - `TaskExecute` 
       - `TaskResume` 
       - `TaskCancel`
   - **Produced Events:**
     - **Commands:**
       - `AgentExecute` 
       - `ToolExecute`
       - `AgentCancel`
       - `ToolCancel`
     - **State Events:**
       - `TaskExecutionSuccess` 
       - `TaskExecutionFailed` 
       - `TaskExecutionCanceled`
       - `TaskWaitingStarted` 
       - `TaskWaitingEnded` 
       - `TaskWaitingTimedOut` 
       - `TaskRetryScheduled` 
     - **Log Events:**
       - `LogEmitted` 

6. **System Runtime**
   - **Package:** `runtime`
   - **Struct:** `Runtime`
   - **Responsibilities:**
     - Provides the execution environment for agents and tools, primarily utilizing Deno.
     - Manages the lifecycle of agent and tool executions, including initialization, execution, cancellation, and cleanup.
     - Handles context and state management for agents and tools during their execution.
     - Executes tool (`ToolExecute`) and agent (`AgentExecute`) implementations when requested by `task.Executor`.
     - Validates inputs and formats outputs for agents and tools according to their schemas.
     - Produces state events (e.g., `AgentExecutionStarted`, `AgentExecutionSuccess`, `AgentExecutionFailed`, `ToolExecutionStarted`, `ToolExecutionSuccess`, `ToolExecutionFailed`).
     - Communicates execution results (success, failure, output) back to the requester (e.g., `task.Executor`).
   - **Consumed Events:**
     - **Commands:**
       - `AgentExecute` 
       - `ToolExecute`
       - `AgentCancel`
       - `ToolCancel`
   - **Produced Events:**
     - **State Events:**
       - `AgentExecutionStarted` 
       - `AgentExecutionSuccess` 
       - `AgentExecutionFailed`
       - `AgentExecutionCanceled`
       - `ToolExecutionStarted` 
       - `ToolExecutionSuccess` 
       - `ToolExecutionFailed`
       - `ToolExecutionCanceled`
     - **Log Events:**
       - `LogEmitted` 

7. **System Monitoring**
   - **Package:** `monitoring`
   - **Struct:** `Monitoring`
   - **Responsibilities:**
     - Collects and processes metrics, logs, and alerts for observability.
     - Consumes state events (e.g., `WorkflowExecutionSuccess`, `TaskExecutionFailed`, `AgentExecutionStarted`, `TaskExecutionCanceled`) to track system performance and health.
     - Consumes log events (`LogEmitted`) for debugging and monitoring.
     - Provides APIs for querying metrics and logs, supporting real-time and historical analysis.
     - Triggers alerts for critical events (e.g., `WorkflowExecutionTimedOut`, `TaskWaitingTimedOut`, `TaskExecutionCanceled`).
     - Tracks and reports on workflow cancellation patterns to identify potential system issues or user experience improvements.
   - **Consumed Events:**
     - **State Events:**
       - `WorkflowExecutionStarted` 
       - `WorkflowExecutionPaused` 
       - `WorkflowExecutionResumed` 
       - `WorkflowExecutionSuccess` 
       - `WorkflowExecutionFailed` 
       - `WorkflowExecutionCanceled` 
       - `WorkflowExecutionTimedOut` 
       - `TaskExecutionStarted` 
       - `TaskExecutionSuccess` 
       - `TaskExecutionFailed` 
       - `TaskExecutionCanceled` 
       - `TaskWaitingStarted` 
       - `TaskWaitingEnded` 
       - `TaskWaitingTimedOut` 
       - `TaskRetryScheduled` 
       - `TaskDispatched` 
       - `AgentExecutionStarted` 
       - `AgentExecutionSuccess` 
       - `AgentExecutionFailed` 
       - `AgentExecutionCanceled` 
       - `ToolExecutionStarted` 
       - `ToolExecutionSuccess` 
       - `ToolExecutionFailed` 
       - `ToolExecutionCanceled` 
     - **Log Events:**
       - `LogEmitted` 
   - **Produced Events:**
     - **Log Events:**
       - `LogEmitted` 

8. **NATS Client**
   - **Package:** `nats`
   - **Struct:** `Client`
   - **Responsibilities:**
     - Facilitates all asynchronous and synchronous communication between workflow engine components.
     - Supports task queues, event streams, and inter-process communication with `system.Runtime`.
     - Enables streaming for real-time updates and chat-style interactions.
     - Ensures durability with JetStream for critical message streams.
   - **Consumed Events:**
     - None (NATS Client handles message transport, not event processing).
   - **Produced Events:**
     - **Log Events:**
       - `LogEmitted` 

9. **NATS Server**
   - **Package:** `nats`
   - **Struct:** `Server`
   - **Responsibilities:**
     - Provides the NATS server for the workflow engine.
     - Manages the NATS server configuration and lifecycle.
     - Handles the NATS server's startup and shutdown.
   - **Consumed Events:**
     - None (NATS Server provides infrastructure, not event processing).
   - **Produced Events:**
     - **Log Events:**
       - `LogEmitted` 

## Impact

These standardized names and event mappings are reflected in:

- Directory structures within `src/internal/`.
- Go package declarations and struct definitions.
- Import paths and instantiation points throughout the codebase.
- References in documentation files (READMEs, architecture diagrams, task definitions, assertion scenarios, etc.).
- Event `Produced By`, `Consumed By`, and `source_component` fields in NATS event documentation.

standardization improves the project's alignment with industry best practices and makes the codebase more intuitive for developers familiar with distributed systems and workflow engine concepts.

## Hierarchical Control Operations

The Compozy Workflow Engine implements control operations (pause, resume, cancel) in a hierarchical manner, ensuring proper propagation throughout the execution tree:

1. **Workflow Cancellation Flow:**
   - API Service receives cancellation request and issues `WorkflowCancel` command
   - System Orchestrator routes this to Workflow Executor
   - Workflow Executor marks the workflow as canceled and issues `TaskCancel` commands for all active tasks
   - Task Executor cancels local execution and issues `AgentCancel`/`ToolCancel` commands as needed
   - System Runtime terminates any running agent/tool processes
   - State Manager records all cancellation events (`WorkflowExecutionCanceled`, `TaskExecutionCanceled`, etc.)

2. **Task Cancellation Flow:**
   - API Service receives task cancellation request and issues `TaskCancel` command
   - Task Executor cancels the specific task and issues `AgentCancel`/`ToolCancel` if needed
   - State Manager records task cancellation event (`TaskExecutionCanceled`)

3. **Pause/Resume Flow:**
   - Similar hierarchical pattern applies to pause/resume operations
   - Pausing a workflow pauses all active tasks
   - Resuming a workflow resumes all paused tasks that were active at pause time

This hierarchical approach ensures consistent system state during control operations and prevents orphaned executions.
