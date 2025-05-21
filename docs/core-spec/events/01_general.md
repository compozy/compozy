# Events Documentation

This document provides a comprehensive reference for NATS events in the Compozy Workflow Engine, merging information from lifecycle mappings, event definitions, and example payloads. Each event includes its purpose, when it's produced and consumed, NATS subject example, and JSON payload example.

## General Principles

### Event Types

The NATS-based communication in Compozy follows a clear separation of concerns using three distinct message types:

1.  **State Events**: Messages that record something that has happened.
    -   Used for event sourcing and maintaining the system state.
    -   Prefixed with "State Event:" in the documentation.
    -   Always consumed by the `state.Manager`.
    -   Follow a specific subject pattern (see NATS Subject Patterns).

2.  **Commands**: Messages that instruct a component to perform an action.
    -   Direct instructions to perform specific work.
    -   Prefixed with "Command:" in the documentation.
    -   Typically consumed by an executor or processor component.
    -   Follow a specific subject pattern (see NATS Subject Patterns).

This separation allows the system to optimize message routing and processing based on purpose.

### Command Communication Patterns

Commands in the system utilize NATS for communication:

1.  **Synchronous Commands (Request-Reply)**:
    -   These commands expect a direct response on a reply subject.
    -   Examples: `TaskExecute`, `AgentExecute`, `ToolExecute`.
    -   Marked as "Communication Pattern: Synchronous (Request-Reply)" in documentation.

2.  **Asynchronous Commands**:
    -   These commands do not expect a direct response. The issuer typically relies on subsequent state events for feedback.
    -   Examples: `WorkflowExecute`, `WorkflowResume`, `WorkflowCancel`, `TaskTrigger`.
    -   Marked as "Communication Pattern: Asynchronous" in documentation.
    -   Feedback Mechanism:
        -   **State Events**: The caller can subscribe to relevant state events (e.g., `WorkflowExecutionStarted` after an `WorkflowExecute` command) to track progress and result.

State events like `ToolExecutionSuccess` are inherently asynchronous notifications.

### Event Structure

All events generally follow a consistent structure with these main sections:

-   **`metadata`**: Common metadata about the event/message itself.
-   **`workflow`**: Information about the workflow (often present).
-   **`task`**: Information about the task (present in task-related events/messages).
-   **`agent`**: Information about the agent (present in agent-related events/messages).
-   **`tool`**: Information about the tool (present in tool-related events/messages).
-   **`agent_runtime`**: Information about the agent runtime (present in agent runtime status events).
-   **`payload`**: Event-specific data.

#### Metadata Section

The `metadata` section typically contains:
-   `correlation_id`: (string, UUID) Identifier to correlate messages across different services or components. This ID is also part of the NATS subject.
-   `event_timestamp`: (string, ISO8601) Time the message was generated.
-   `source_component`: (string) Name of the component that generated the message.
-   `created_by`: (string, optional) Identifier of the user or system that triggered the event.

#### Entity Sections (`workflow`, `task`, `agent`, `tool`, `agent_runtime`)

These sections contain identifiers and key information for the respective entities:
-   `id`: (string) Identifier for the entity definition (e.g., `workflow.id`, `task.id`).
-   `exec_id`: (string, UUID) Unique identifier for a specific run/instance of the entity.
-   `name`: (string, optional) Human-readable name of the entity.

#### Standardized Payload Section (for State Events and Command Results)

The `payload` section for state events and command results aims for a standardized structure:

-   **`status`** (`string`, Required)
    -   Description: The primary status indicated by the event (e.g., `"RUNNING"`, `"COMPLETED"`, `"FAILED"`, `"WAITING"`, `"ACKNOWLEDGED"`). For command results, this reflects the result of the command execution (e.g., `"SUCCESS"`, `"TOOL_ERROR"`).
-   **`result`** (`object`, Optional)
    -   Description: Contains the result of the operation, either an `output` or an `error`. This field is typically present if the event represents a terminal state or a direct result.
    -   Properties:
        -   **`output`** (`object`, Optional): Structured data representing the successful result of the operation. Present if the operation was successful and produced data.
        -   **`error`** (`object`, Optional): Structured data detailing an error if the operation failed. Present if the operation resulted in an error.
            -   `message` (`string`, Required): A human-readable error message.
            -   `code` (`string`, Optional): A machine-readable error code (e.g., `"TASK_FAILED"`, `"API_TIMEOUT"`, `"WAIT_TIMEOUT"`).
            -   `details` (`object`, Optional): Additional structured details about the error.
-   **`duration_ms`** (`integer`, Optional)
    -   Description: The duration of the operation or a relevant time period in milliseconds.
-   **`context`** (`object`, Optional)
    -   Description: An object containing additional event-specific data that doesn't fit into `status` or `result`. This can include reasons, configuration details, identifiers (like `state_id`), or other contextual information.
    -   Usual Properties:
        -   **`reason`** (`string`, Optional): A human-readable reason for the event.
        -   **`timeout_config`** (`object`, Optional): The timeout configuration for the operation.
        -   **`input`** (`object`, Optional): The input data for the operation.
        -   **`env`** (`object`, Optional): The environment variables for the operation.
        -   **`agent_request`** (`object`, Optional): The agent request for the operation.

### Common Enums (Conceptual)

While not strictly enforced in JSON, these represent common status values:

```
// Workflow Status Values
"PENDING", "RUNNING", "PAUSED", "SUCCESS", "FAILED", "TIMED_OUT", "CANCELED"

// Task Status Values
"PENDING", "RUNNING", "WAITING", "SUCCESS", "FAILED", "TIMED_OUT", "CANCELED"

// Tool Execution Status Values
"RUNNING", "SUCCESS", "FAILED"

// Agent Status Values
"RUNNING", "SUCCESS", "FAILED"

// Agent Runtime Status Values
"REGISTERED", "DEREGISTERED"

// Command Result Status Values
"SUCCESS", "FAILURE", "ERROR", "TOOL_ERROR" (more specific codes often in result.error.code)
```

Specific events will define their applicable status values.
