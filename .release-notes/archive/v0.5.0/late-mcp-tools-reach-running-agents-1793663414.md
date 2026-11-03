---
title: MCP tools that arrive late reach running agents, and tool lists load faster
type: fix
---

An agent could finish listing its MCP tools before a server's tools were discovered, then never see them. CompozyOS now tells connected providers when the list changes, so they can pick up and call tools that arrive late. This was tested directly against the Claude adapter, not through a full session.

Listing tools also no longer rediscovers every MCP server once per tool. With two slow local servers and 114 tools, the list went from 36 seconds to under a second. Other causes of a slow list may remain, since this fixes the one we reproduced.
