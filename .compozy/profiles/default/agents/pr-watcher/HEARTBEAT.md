---
version: 1
enabled: true
summary: "Re-verify watched pull requests only when a monitor wake was lost."
preferences:
  min_interval: "2h"
  active_hours:
    - timezone: "America/Sao_Paulo"
      start: "08:00"
      end: "22:00"
context:
  include: [monitors_fired]
---
