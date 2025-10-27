# Builder Outline: Agent & Action
- **Purpose:** Cover `sdk/agent` builder and nested `ActionBuilder`. Source: tasks/prd-sdk/03-sdk-entities.md §"Agent Definition".
- **Audience:** Engineers configuring autonomous agents and actions.
- **Sections:**
  1. Agent configuration (models, memory, tools) summarizing method list.
  2. ActionBuilder usage (transitions, retry, guardrails) referencing 03-sdk-entities.md action subsection.
  3. Prompting best practices referencing 05-examples.md agent prompts.
  4. Monitoring & analytics hooks referencing 02-architecture.md observability.
  5. Validation and error handling (BuildError fields) referencing 03-sdk-entities.md.
- **Content Sources:** 03-sdk-entities.md, 05-examples.md §§ multi-agent pipeline, _techspec.md prompting guardrails.
- **Cross-links:** Tasks builder page (task invocation), Knowledge builder (retrieval), Core agent concepts doc.
- **Examples:** `sdk/examples/06_multi_agent_pipeline.go`, `05_action_retry.go`.
