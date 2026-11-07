---
title: MCP servers reach your open sessions
type: fix
---

Adding or signing in to an MCP server could show it as Ready in Settings while a session that was already open still reported it as unavailable. MCP servers declared in your user or profile `mcp.json` could also be missing from new sessions. Open sessions now pick up an MCP server as soon as you add or authorize it, and lose access when you sign out, disable or remove it, without restarting CompozyOS or losing the conversation. Claude sessions refresh their tool list in place. Codex sessions reload at the next turn boundary, because Codex does not yet re-list MCP tools mid-session.
