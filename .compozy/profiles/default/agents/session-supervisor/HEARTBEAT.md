---
version: 1
enabled: true
summary: "Inspect the orchestrator's session tree for stalls and act only on new ones."
preferences:
  min_interval: "15m"
  active_hours:
    - timezone: "America/Sao_Paulo"
      start: "08:00"
      end: "22:00"
context:
  include: [session_health, sessions_idle, monitors_fired]
---
