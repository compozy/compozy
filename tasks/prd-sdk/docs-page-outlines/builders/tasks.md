# Builder Outline: Tasks (9 Types + Signal System)
- **Purpose:** Document `sdk/task` builders covering all nine task types and signal handling. Source: tasks/prd-sdk/03-sdk-entities.md §"Task Creation".
- **Audience:** Workflow authors configuring task nodes.
- **Sections:**
  1. Overview table of task types (Action, ToolCall, WorkflowCall, Condition, Parallel, Iteration, ScheduleTrigger, Webhook, Function) referencing 03-sdk-entities.md matrix.
  2. Fluent configuration snippets for each builder (parameters, context requirements).
  3. Signal system integration (`sdk/task.Signal`) representing the 16th category; explain event-driven workflows referencing 03-sdk-entities.md signal notes.
  4. Error handling & retries referencing _techspec.md retry policies.
  5. Validation checklist linking to testing guide for unit coverage.
- **Content Sources:** 03-sdk-entities.md, 05-examples.md (parallel, tool call, webhook examples), 02-architecture.md task execution flow.
- **Cross-links:** Workflow builder page, CLI monitoring commands, troubleshooting page for task failures.
- **Examples:** `sdk/examples/02_parallel_tasks.go`, `03_tool_call.go`, `04_model_failover.go`, `07_signal_router.go`.
