---
id: RT-home-usage-window-persistence
area: RT
title: Usage window and system fold persist across reloads
persona: End user
journey:
expected: Selecting 7d/30d/90d in Usage & cost refetches the overview with `usage_window` and re-renders totals + chart; the choice and the System row fold state survive a full reload (localStorage `agh:home-prefs`); a window larger than `observability.retention_days` sets `truncated` and renders the retention footnote; cost figures render only with truthful provenance (mixed provenance → no cost, status unknown).
entry_points: web `/` Usage & cost zone + System row; `GET /api/observe/overview?usage_window=`
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: web/src/systems/dashboard/hooks/use-home-prefs-store.ts; web/src/systems/dashboard/components/home-usage-chart.tsx; internal/observe/overview_usage.go
last_report:
overlaps:
---

story: As an end user my usage window choice and the folded operator row stay where I left them.

New behavior shipped 2026-07-23. Daily token buckets accrue from the `token_usage_daily` rollup (migration 00026) starting at deploy — early charts are honestly sparse; retention sweeps prune rollup days with the same `observability.retention_days` policy as events.
