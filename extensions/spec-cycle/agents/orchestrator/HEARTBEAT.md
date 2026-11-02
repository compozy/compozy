---
context:
  include: [session_health, sessions_idle, monitors_fired, work_log]
---

Read the workspace-root `.compozy/work-log.md` and correlate stale accepted entries with current session and monitor facts. Report or propose dispatching or closing entries older than 30 minutes; use evidence before proposing closure. Flag `work:more` and `work:log-truncated` so hidden open work is inspected. Triage is read-only: do not edit the log or dispatch work in this turn. Keep proposals within the triage limit. User and global instances have no workspace work log.
