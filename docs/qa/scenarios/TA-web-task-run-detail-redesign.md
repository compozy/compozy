---
id: TA-web-task-run-detail-redesign
area: TA
title: Run detail attempt page with outcome band and session rail
persona: Bruno
journey: J-complete-task-tree
expected: Run detail renders the Tasks / <task> / Attempt N head (Retry primary only while attempts remain; Open session otherwise; overflow holds release/cancel/recover/force-fail with required reason), a tinted outcome band with failure reason + event chip, Result, Reviews as round cards keyed by review_id, a live Run activity feed scoped to the attempt, and the session rail (Session/Timing/Lineage + agh task run show hint). Metrics the runtime did not report render "—".
entry_points: web /tasks/:id/runs/:runId; run Inspect drawer
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-019; TA-terminal-run-inspect; NB-run-conversation-bounds-usage
---

Introduced by the opendesign tasks redesign (docs/design/opendesign/tasks/task-run-detail.html, implemented 2026-07-21). Visual contract evidence: .compozy/tasks/os-shell/evidence/visual/opendesign-redesigns/VC-T2/.
