---
id: LP-task-rollup-wakes-loop
area: LP
title: Wake one Loop from a parent task rollup
persona: Bruno
journey: J-complete-task-tree
expected: The parent completion event wakes the enabled matching Loop and creates exactly one follow-up task; a disabled Loop and unrelated workspace task tree do not match.
entry_points: web Loop editor; web task detail; web Loop run detail
qa_status: blocked-verify
bug_ids: BUG-20260713-parent-task-rollup-missing; BUG-20260713-task-role-session-never-starts
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/compozy71-all-children-completed-parent-stuck.dom.txt;/Users/pedronauck/dev/qa-labs/compozy-lp-public-interface-20260730-060347-933555-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: LP-042
---

The parent rollup event is the product boundary between Compozy-71 and Loop
watch/trigger behavior.

QA impact 2026-07-14: parent completion wake publication now survives request cancellation and dependent-reconciliation failure. The matching positive case plus disabled-Loop and unrelated-workspace negative controls remain required before promotion.

2026-07-14 automated final-worktree control: the complete runtime E2E gate passed the matching parent-completion wake and both disabled-Loop and unrelated-workspace negative controls. Retest promoted to pass.

2026-07-21: qa_status reset to untested — the opendesign redesigns restructured this scenario's web entry surface (task detail/run detail 3-tab IA, settings takeover shell, or providers page); the pass verdict predates that surface.
