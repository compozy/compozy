---
name: invalid-hook
description: Skill with invalid hook event
metadata:
  compozy:
    hooks:
      - event: foo.bar
        command: /bin/echo
        args:
          - invalid
---

body
