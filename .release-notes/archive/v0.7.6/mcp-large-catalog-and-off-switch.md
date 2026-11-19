---
title: Keep sessions responsive with large MCP catalogs and turn MCP servers off
type: fix
---

Sessions stay responsive when your MCP servers expose large tool catalogs. Starting a session no
longer spends its launch budget discovering upstream catalogs: it binds to the catalog CompozyOS
already holds, and a background worker refreshes servers on its own. Idle sessions no longer list
tools again just to renew a cache, and sessions that see the same tools share one copy of the
catalog instead of each keeping its own. A server whose discovery fails keeps its last successful
tools and reports why it failed.

A server with a live `tools/list_changed` subscription refreshes when it reports a change. For a
server without one, CompozyOS runs one shared refresh per server and credential binding while an
authorized session is bound. The interval is at least 60 seconds, or the server's `ttlMs` hint if
that is longer, and two minutes when the server gives no hint. A failed refresh waits the same
interval and keeps the last good catalog. Refreshing stops when the last session using the server
ends. **Check again** still contacts the server immediately.

A stdio server is no longer treated as failing, and restarted, when the caller's own deadline ran
out. CompozyOS now times the server's response after it accepts the call. When a session bind is
slow, the daemon log shows how long each phase took.

You can now turn a configured MCP server off without deleting it. Use the switch on the server's
row in **Settings → MCP servers**, or the CLI:

```bash
compozy mcp disable example --scope workspace --workspace my-project
compozy mcp enable example --scope workspace --workspace my-project
```

The verbs take the same scope selectors as the other `compozy mcp` commands. For a server an
extension provides, pass `--owner extension:<installed-name>`. You can also set `disabled = true` in
a `[[mcp_servers]]` entry in `config.toml`, or `"disabled": true` in an `mcp.json` server object.
Over HTTP and UDS the operation is `PUT /api/settings/mcp-servers/{name}/enabled` with a body such as
`{"enabled":false}`.

An off server keeps its definition, credentials and approval. It contributes no tools, and
CompozyOS skips connecting to it, discovering its tools, checking it and renewing its credentials.
The session's **MCP servers** control lists it as **Off**. Open sessions pick up a change on their
next message. An off server declared in a narrower scope still takes precedence over a server with
the same name in a broader scope, so turning it off does not fall back to the broader one. Servers
with no configured value stay on, so existing configuration keeps working unchanged.
