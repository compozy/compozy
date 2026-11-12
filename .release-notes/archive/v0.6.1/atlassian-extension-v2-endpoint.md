---
title: The Atlassian extension uses Atlassian's v2 MCP endpoint
type: fix
---

The Atlassian extension 1.0.1 connects to `https://mcp.atlassian.com/v2/mcp`, the endpoint Atlassian recommends. On some sites, Jira, Confluence and Rovo Search calls through the old endpoint failed with `401 Unauthorized; scope does not match` even after a successful sign-in.

Migration notes: update the Atlassian extension to 1.0.1 from the Marketplace and sign in again when prompted.
