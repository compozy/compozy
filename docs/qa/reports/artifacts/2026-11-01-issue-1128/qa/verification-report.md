# Verification report - monitor durability lab (#1128, PR #1129)

Build under test: commit e6e92dea83a2ccae1a67f8323ecd2242e05f8abd (lab binaries built from it).

Verdict: PASS for the three scoped journeys. The strict auditor still reports C14 (no local gate run) because
the operator forbids local make gate/verify for this work; required CI on the PR head is the gate.

Journeys (raw per-step output in the evidence directories, rows in journey-log.jsonl, all 48 steps exit 0):
- evidence-external-wait: PASS. About 3m22s idle wait on an external marker, duplicate registration coalesced,
  exactly one monitor_exit wake, monitor fired exit 0.
- evidence-restart-eligible: PASS. Daemon restart via POST /api/settings/actions/restart reached ready. The
  terminal monitor fired terminal_unavailable after the daemon returned; the timer fired on its original deadline;
  the operator-stopped session's monitor is canceled and the session was not resumed or woken.
- evidence-restart-ineligible: PASS. session.restart.auto_continue=false: both monitors interrupted
  (owner_disabled), listed over HTTP, no wake and no resume after the timer deadline.

Limits: acpmock agents with the real hosted native tools; no live provider and no real GitHub watch.
