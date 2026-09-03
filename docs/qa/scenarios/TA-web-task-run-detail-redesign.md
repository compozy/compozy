---
id: TA-web-task-run-detail-redesign
area: TA
title: Run detail attempt page with outcome band and session rail
persona: Bruno
journey: J-complete-task-tree
expected: Run detail renders the Tasks / <task> / Attempt N head (Retry primary only while attempts remain; Open session otherwise; overflow holds release/cancel/recover/force-fail with required reason), a tinted outcome band with failure reason + event chip, Result, Reviews as round cards keyed by review_id, a live Run activity feed scoped to the attempt, and the session rail (Session/Timing/Lineage + compozy task run show hint). Metrics the runtime did not report render "—".
entry_points: web /tasks/:id/runs/:runId; run Inspect drawer
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-ta-replay-20260730-062156-531636-lab/qa-artifacts/qa; docs/qa/reports/2026-08-28-integrated-terminal-rebase.md
last_report: docs/qa/reports/2026-08-28-integrated-terminal-rebase.md
overlaps: TA-019; TA-terminal-run-inspect; NB-run-conversation-bounds-usage
---

Introduced by the opendesign tasks redesign (docs/design/opendesign/tasks/task-run-detail.html, implemented 2026-07-21). Visual contract evidence: .compozy/tasks/os-shell/evidence/visual/opendesign-redesigns/VC-T2/.

2026-08-30 CI repair re-walk: passed. Exact-head CI run `33296083331` exposed a loaded
session permalink that did not complete its route-to-window handoff under lane load. The run detail
now uses its owned `session_id` and `agent_name` to open the canonical session route directly, with
the ID-only permalink retained for incomplete projections. The shipped Tasks browser journey passed
5/5 focused repetitions after the repair.
