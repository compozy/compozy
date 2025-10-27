# Builder Outline: Schedule
- **Purpose:** Explain `sdk/schedule` builder for time-based triggers. Source: tasks/prd-sdk/03-sdk-entities.md §"Schedule Configuration".
- **Audience:** Operators configuring scheduled workflows.
- **Sections:**
  1. Schedule types (cron, interval, calendar) referencing method list.
  2. Binding schedules to workflows referencing 03-sdk-entities.md usage.
  3. Timezone, daylight saving considerations referencing 04-implementation-plan.md operations notes.
  4. Monitoring & overrides referencing 02-architecture.md operations.
  5. Testing schedules referencing 07-testing-strategy.md scheduled tests.
- **Content Sources:** 03-sdk-entities.md, 04-implementation-plan.md, 07-testing-strategy.md.
- **Cross-links:** Workflow builder page, runtime builder (scheduler workers), CLI schedule commands.
- **Examples:** `sdk/examples/08_scheduler.go`.
