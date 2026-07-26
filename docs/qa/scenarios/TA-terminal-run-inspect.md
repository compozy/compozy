---
id: TA-terminal-run-inspect
area: TA
title: Keep terminal task-run diagnostics terminal
persona: Bruno
journey: J-complete-task-tree
expected: A completed or otherwise terminal run never emits an active orphan diagnostic or a release recovery command merely because its formerly bound session is terminal.
entry_points: Web Task inspect diagnostics; Web run detail; structured Task inspect
qa_status: untested
bug_ids: BUG-20260714-terminal-task-run-reported-orphan
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/agh71-faithful-child-b-one-run.dom.txt;/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/terminal-task-run-no-orphan-fixed.dom.txt
last_report: docs/qa/reports/2026-07-13-automation-features.md
overlaps: TA-task-role-session-activation
---

The terminal Task status, run status, diagnostics, and suggested recovery must agree after a real worker session exits.

2026-07-14 retest: the original completed run retained its stopped-session audit context but both Task and run detail rendered zero diagnostics, terminal next action, and no release command after a real daemon rebuild/reload.

2026-07-21: qa_status reset to untested — the opendesign redesigns restructured this scenario's web entry surface (task detail/run detail 3-tab IA, settings takeover shell, or providers page); the pass verdict predates that surface.
