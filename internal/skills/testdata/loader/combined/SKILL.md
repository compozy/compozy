---
name: combined
description: Skill with MCP and hook metadata
metadata:
  agh:
    mcp_servers:
      - name: git
        command: uvx
        args:
          - mcp-server-git
        env:
          REPO_ROOT: "${REPO_ROOT}"
    hooks:
      - event: session.post_stop
        command: /usr/bin/env
        args:
          - bash
          - -lc
          - echo cleanup
        timeout: 30s
        env:
          PHASE: stop
---

body
