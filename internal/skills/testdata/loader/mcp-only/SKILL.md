---
name: mcp-only
description: Skill with MCP metadata only
metadata:
  compozy:
    mcp_servers:
      - name: filesystem
        command: npx
        args:
          - -y
          - "@modelcontextprotocol/server-filesystem"
        env:
          ROOT: "${WORKSPACE_ROOT}"
          MODE: read-only
---

body
