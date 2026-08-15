---
id: LP-terminal-outcome-notification
area: LP
title: Deliver the effect for the terminal outcome that actually committed
persona: Lea
journey: J-recover-loop-node-failure
expected: Exactly the declared effect for the committed done, no-op, blocked, failed, exhausted, stalled, or canceled outcome is delivered once, and its result is observable without firing another lifecycle hook.
entry_points: web /loops/:name/editor; web /loop-runs/:id; `compozy loop status --run-id <run-id> -o json`; HTTP/UDS Loop events; SSE
qa_status: pass
bug_ids: compozy/compozy#403
fix_status: fixed
retest_status: pass
fix_commits: b3c0087f
evidence: docs/qa/evidence/2026-08-14-loop-effects-combined/success-reload.png; docs/qa/evidence/2026-08-14-loop-effects-combined/denied-reload.png
last_report: docs/qa/reports/2026-08-14-loop-effect-tool-policy.md
overlaps:
---

Fresh isolated QA confirmed the permitted and foreign-workspace-denied terminal-effect paths
through CLI, runtime, API/SSE, and Web before and after reload. The visual read path used the
separately stacked Web presentation fix; the daemon policy verdict belongs to compozy/compozy#403.
Prior terminal outcome evidence is available in `docs/qa/reports/2026-08-03-loop-node-lifecycle.md`.

acceptance-walk: Seed separate runs that commit done, no-op, blocked, failed, exhausted, stalled, and canceled, with distinct terminal effects. For the done run, declare a permitted read-only native tool effect that explicitly targets the run workspace. Confirm each run delivers only its committed terminal effect once; the done result is `outcome: ok` with no authored/public `loop-effect` agent; a foreign-workspace target is denied; and refreshed Web, structured CLI, and HTTP event reads expose the same delivery result without recursive lifecycle effects.
