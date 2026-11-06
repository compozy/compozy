---
version: 1
enabled: true
summary: "Correlate stale work-log entries with session health; session monitors already wake this agent for its children."
preferences:
  min_interval: "60m"
  active_hours:
    - timezone: "America/Sao_Paulo"
      start: "08:00"
      end: "22:00"
context:
  include: [work_log, session_health]
---

Read the workspace-root `.compozy/work-log.md` and correlate stale accepted entries with current session health facts. Report or propose dispatching or closing entries older than 30 minutes; use evidence before proposing closure. Flag `work:more` and `work:log-truncated` so hidden open work is inspected. Triage is read-only: do not edit the log or dispatch work in this turn. Keep proposals within the triage limit. User and global instances have no workspace work log.
