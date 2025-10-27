# Builder Outline: Workflow
- **Purpose:** Detail `sdk/workflow` builder and related configuration. Source: tasks/prd-sdk/03-sdk-entities.md §"Workflow Construction".
- **Audience:** Workflow authors designing execution graphs.
- **Sections:**
  1. Workflow anatomy (metadata, tasks, transitions) referencing method list.
  2. Task orchestration patterns (sequential, fan-out, conditions) referencing 03-sdk-entities.md orchestration notes.
  3. Validation & namespacing rules referencing validation subsection.
  4. Integration with schedule, monitoring, runtime.
  5. Tips for large workflows (link to architecture doc for engine constraints).
- **Content Sources:** 03-sdk-entities.md, 04-implementation-plan.md workflow registration steps.
- **Cross-links:** Tasks builder page, Core workflow DSL doc, CLI execution doc.
- **Examples:** Link to `sdk/examples/02_parallel_tasks.go`, `03_tool_call.go`, `06_multi_agent_pipeline.go`.
