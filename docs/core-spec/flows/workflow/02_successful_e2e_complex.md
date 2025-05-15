# Flow: Successful Workflow Execution (Complex E2E)

This diagram illustrates a more complex workflow with multiple tasks, including one that calls an agent and another that calls a tool. All operations are successful.

```mermaid
sequenceDiagram
    participant Client as Client/API/Trigger
    participant WFEngine as Workflow Engine (Orchestrator)
    participant Task1Path as Task 1 Execution Path (calls Agent)
    participant Task2Path as Task 2 Execution Path (calls Tool)
    participant SM as State Manager
    participant NATS

    Client->>NATS: `WorkflowExecute`
    NATS-->>WFEngine: Delivers `WorkflowExecute`

    WFEngine->>NATS: `WorkflowExecutionStarted`
    NATS-->>SM: Records `WorkflowExecutionStarted`

    Note over WFEngine: Starts Task 1 (Agent-calling task)
    WFEngine->>Task1Path: Initiates Task 1 (see `../task/03_successful_execution_with_agent.md`)
    activate Task1Path
    Note over Task1Path: Task 1 schedules, starts, invokes Agent successfully, completes.
    Task1Path-->>WFEngine: Task 1 `TaskExecutionResult`
    deactivate Task1Path

    Note over WFEngine: Processes Task 1 completion, starts Task 2 (Tool-calling task)
    WFEngine->>Task2Path: Initiates Task 2 (see `../task/02_successful_execution_with_tool.md`)
    activate Task2Path
    Note over Task2Path: Task 2 schedules, starts, invokes Tool successfully, completes.
    Task2Path-->>WFEngine: Task 2 `TaskExecutionResult`
    deactivate Task2Path

    Note over WFEngine: All tasks completed successfully.
    WFEngine->>NATS: `WorkflowExecutionSuccess`
    NATS-->>SM: Records `WorkflowExecutionSuccess`
    NATS-->>Client: (Optional) Notify Client of completion

```

This flow involves:
1.  `Client` sends `WorkflowExecute`.
2.  `Workflow Engine` (WFEngine) emits `WorkflowExecutionStarted`.
3.  **Task 1 Execution (Agent Call):**
    *   `WFEngine` initiates Task 1.
    *   Task 1 executes, successfully calling an agent (details in `../task/03_successful_execution_with_agent.md`).
    *   Task 1 emits `TaskExecutionResult`, consumed by `WFEngine`.
4.  **Task 2 Execution (Tool Call):**
    *   `WFEngine` initiates Task 2 based on workflow logic (e.g., after Task 1 completion).
    *   Task 2 executes, successfully calling a tool (details in `../task/02_successful_execution_with_tool.md`).
    *   Task 2 emits `TaskExecutionResult`, consumed by `WFEngine`.
5.  `WFEngine`, determining all steps are complete, emits `WorkflowExecutionSuccess`.
6.  `State Manager` records all state changes throughout. 
