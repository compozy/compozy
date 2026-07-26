---
name: invalid-hook-command
description: Skill with missing hook command
metadata:
  compozy:
    hooks:
      - event: session.post_create
        args:
          - invalid
---

body
