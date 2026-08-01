---
id: RT-home-usage-window-persistence
area: RT
title: Usage window and system fold persist across reloads
persona: Cora
journey: J-operate-home-dashboard
expected: Selecting 7d/30d/90d in Usage & cost refetches the overview with `usage_window` and re-renders totals + chart; the choice and the System row fold state survive a full reload (localStorage `compozy:home-prefs:v2`); a window larger than `observability.retention_days` sets `truncated` and renders the retention footnote; cost figures render only with truthful provenance (mixed provenance → no cost, status unknown).
entry_points: web `/` Usage & cost zone + System row; `GET /api/observe/overview?usage_window=`
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/home-usage-30d-system-expanded.png
last_report: docs/qa/reports/2026-08-01-window-tabs.md
overlaps:
---

story: As an end user my usage window choice and the folded operator row stay where I left them.

New behavior shipped 2026-07-23. Daily token buckets accrue from the `token_usage_daily` rollup (migration 00026) starting at deploy — early charts are honestly sparse; retention sweeps prune rollup days with the same `observability.retention_days` policy as events.

QA impact 2026-07-25: Home preferences moved to XState Store and hard-cut to `compozy:home-prefs:v2`; the previous client envelope is intentionally not read. Usage queries and rendered behavior are unchanged. Status remains untested; no QA replay ran.

QA completion 2026-07-29: 7d, 30d, and 90d each issued the matching overview request and updated
the chart label. With seven retained days, 7d omitted the truncation note while 30d and 90d rendered
the exact retention boundary. `cost_status=unknown` produced no numeric cost. A full reload preserved
90d plus the expanded System panel through `compozy:home-prefs:v2`; the browser-local key was restored
to its original absent state after the run.

QA impact 2026-07-31: session-tab attention and observability now refresh provider/model facts from
the authoritative session event. Reset as the adjacent Home usage canary.
