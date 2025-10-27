# Builder Outline: Memory
- **Purpose:** Cover `sdk/memory` builders (Config and Reference). Source: tasks/prd-sdk/03-sdk-entities.md §"Memory System".
- **Audience:** Engineers configuring conversational or long-term memory.
- **Sections:**
  1. Memory configuration options (storage, retention, privacy) referencing method list.
  2. Reference builder usage (attaching memory to agents/tasks) referencing 03-sdk-entities.md.
  3. Sync & eviction strategies referencing 02-architecture.md memory lifecycle.
  4. Testing patterns referencing 07-testing-strategy.md memory tests.
  5. Operational best practices (monitoring, scaling) referencing _techspec.md.
- **Content Sources:** 03-sdk-entities.md, 02-architecture.md, 07-testing-strategy.md.
- **Cross-links:** Knowledge page, agent page, troubleshooting (memory drift).
- **Examples:** `sdk/examples/05_knowledge_search.go`, `06_multi_agent_pipeline.go`.
