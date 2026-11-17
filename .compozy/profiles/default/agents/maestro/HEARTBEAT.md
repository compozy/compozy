---
version: 1
enabled: true
summary: "Correlate stale work-log entries with session health; never assume a child is covered by a session monitor."
preferences:
  min_interval: "60m"
context:
  include:
    - "work_log"
    - "session_health"
---

Read the workspace-root `.compozy/work-log.md` and correlate stale accepted entries with current session health facts. For each dispatched `[~]` entry, check that its recorded session is still working or has a live monitor; a settled child whose `pr:` or `blocked:` report nobody has acted on is a finding. Report or propose dispatching or closing entries older than 30 minutes; use evidence before proposing closure. Flag `work:more` and `work:log-truncated` so hidden open work is inspected. Triage is read-only: do not edit the log or dispatch work in this turn. Keep proposals within the triage limit. User and global instances have no workspace work log.
