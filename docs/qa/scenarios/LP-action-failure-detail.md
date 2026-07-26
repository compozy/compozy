---
id: LP-action-failure-detail
area: LP
title: Explain a failed Loop action with its preserved cause and recovery path
persona: Lea
journey: J-01
expected: A failed action node preserves and renders the actionable backend cause, and a terminal stalled run tells the operator what to correct before retrying.
entry_points: web Loop run detail; GET /api/workspaces/:workspace_id/loop-runs/:run_id
qa_status: untested
bug_ids: BUG-20260713-loop-failure-hidden
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-001-software-delivery-stalled-missing-taskset.png; /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-001-loop-failure-detail-fixed.dom.txt
last_report: docs/qa/reports/2026-07-13-automation-features.md
---

story: As a first-time Loop operator, I can distinguish a correctable input or workspace prerequisite from a broken Loop, extension, or provider.

truthful-ui: Terminal status alone is insufficient; the visible failed node must preserve the real cause rather than replace it with `loop_action_failed` or a generic backend failure.

e2e: Owning Loop persistence/projection suite plus a browser replay of a bundled action failing with a deterministic validation error.

2026-07-13: Failed in CH-001. `software-delivery` stalled after two `load_tasks` attempts, while neither the run detail nor its persisted projection exposed the missing task-pattern cause.

2026-07-13: Passed same-persona retest in browser-created run `looprun-b165c15b174e3d40`. Both failed generations rendered the bounded missing-pattern cause and concrete retry guidance, and the public run API persisted the structured `action_failure` payload.

2026-07-21: qa_status reset to untested — the opendesign redesigns restructured this scenario's web entry surface (task detail/run detail 3-tab IA, settings takeover shell, or providers page); the pass verdict predates that surface.
