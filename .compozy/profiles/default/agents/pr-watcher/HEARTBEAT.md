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

On recovery, repeat AGENT.md verification, including semantic hygiene on the first wake and after every body edit. Re-read the body even when the head is unchanged. Send `pr-hygiene: #<pr> <findings>` to the executor and hold `ready:` until hygiene and the current-head `PR body` check pass.
