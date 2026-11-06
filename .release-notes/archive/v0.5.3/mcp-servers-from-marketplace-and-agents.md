---
title: Sign in to MCP servers, attach them to agents, and find them in the Marketplace
type: highlight
---

MCP servers are now easier to reach and connect, wherever you start.

- **Sign in to any OAuth remote.** A plain HTTP MCP remote that reported **Needs sign-in** used to offer only Edit, and `compozy mcp login` rejected it. Remotes without custom headers now discover their authorization server and register a client automatically, so **Sign in** just works from Settings and the CLI. The Advanced editor has a working **Use OAuth** toggle. Remotes that authenticate with headers are left alone.
- **Attach MCP servers to an agent.** An agent's Tools sheet showed its MCP servers but could not add one. It now offers **Add MCP server**: pick a configured server or create a new connection. Configured servers are saved by name in the agent's `mcp.json`, so they follow later edits to that server. If the server is removed, it shows **Broken reference**. Secrets stay in your Vault.
- **MCPs in the Marketplace.** A **Plugins | MCPs** toggle next to the Marketplace search opens your MCP servers with the same status, sign-in, edit and remove actions as Settings, plus **Add MCP server**.
