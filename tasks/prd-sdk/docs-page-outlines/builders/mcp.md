# Builder Outline: MCP Integration
- **Purpose:** Document `sdk/mcp` configuration for Model Context Protocol integrations. Source: tasks/prd-sdk/03-sdk-entities.md §"MCP Integration".
- **Audience:** Developers connecting external tools and data providers via MCP.
- **Sections:**
  1. MCP configuration overview (transports, headers, sessions) referencing method list.
  2. Security considerations (auth, token handling) referencing _techspec.md security requirements.
  3. Error handling & retry strategies referencing task_55.md MCP failure modes.
  4. Deployment & operations (diagnostics commands) referencing 02-architecture.md integration notes.
  5. Example integration walkthrough linking to MCP example in 05-examples.md.
- **Content Sources:** 03-sdk-entities.md, 05-examples.md §"MCP Proxy", task_55.md.
- **Cross-links:** Runtime builder (native tools), Tool builder page, CLI diagnostics doc.
- **Examples:** `sdk/examples/07_mcp_proxy.go`.
