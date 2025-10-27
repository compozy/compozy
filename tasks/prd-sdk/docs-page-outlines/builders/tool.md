# Builder Outline: Tool
- **Purpose:** Cover `sdk/tool` builder for custom tool definitions. Source: tasks/prd-sdk/03-sdk-entities.md §"Tool Definition".
- **Audience:** Engineers registering sync/async tools.
- **Sections:**
  1. Tool metadata & handlers (sync, async, streaming) referencing method list.
  2. Input/output schema linkage referencing schema builder.
  3. Security & sandboxing considerations referencing _techspec.md tool guardrails.
  4. Versioning & lifecycle referencing 04-implementation-plan.md tool registry tasks.
  5. Testing tools (unit + integration) referencing 07-testing-strategy.md.
- **Content Sources:** 03-sdk-entities.md, 05-examples.md tool call example, _techspec.md security.
- **Cross-links:** Schema builder page, tasks builder (ToolCall), troubleshooting page.
- **Examples:** `sdk/examples/03_tool_call.go`, `04_model_failover.go`.
