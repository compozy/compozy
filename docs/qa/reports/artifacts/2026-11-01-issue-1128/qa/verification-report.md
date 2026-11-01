# Verification report - monitor durability lab (#1128, PR #1129)

Verdict: PASS for the three scoped journeys; the strict auditor still reports C14 (no local gate run) because
the operator forbids local `make gate`/verify for this work; required CI on the PR head is the gate.

Journeys (raw per-step output in the evidence directories, rows in journey-log.jsonl):
- evidence-external-wait: PASS. About 3m21s idle wait on an external marker, duplicate registration coalesced
  (one `monitor.created`, one `monitor.reused`), exactly one `monitor_exit` wake, monitor `fired` exit 0.
- evidence-restart-eligible: PASS. Daemon restart via POST /api/settings/actions/restart. Terminal monitor fired
  `terminal_unavailable` after the daemon returned; timer fired on its original deadline; operator-stopped
  session's monitor `canceled` and the session was not resumed or woken.
- evidence-restart-ineligible: PASS. `session.restart.auto_continue=false`: both monitors `interrupted`
  (`owner_disabled`), listed over HTTP, no wake and no resume after the timer deadline.
- evidence-run1..run4: earlier walks that found two defects (observer read a cleared stop classification;
  degraded stop-time binding). Both fixed; kept as defect evidence, not as proof of the final build.

Limits: acpmock agents with the real hosted native tools; no live provider and no real GitHub watch.
