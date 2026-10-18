---
id: TA-web-task-detail-redesign
area: TA
title: Task detail 3-tab IA with command-state head
persona: Bruno
journey: J-complete-task-tree
expected: Task detail renders Overview/Runs/Activity tabs with the 48px drill-in head (back, Tasks / <task> trail, status pill, one primary action from the §6 command machine — recover > publish > approve > resume > open run > retry > start — plus overflow verbs), an outcome/now strip matching the task state, subtasks with stacked progress, the 320px properties rail (priority + auto-enqueue editable; owner read-only), and the Inspect drawer (Diagnostics/Stream/Raw). Nullable metrics render "—"; no set-status or delete-run control exists anywhere.
entry_points: web /tasks/:id (Overview, Runs, Activity tabs); Inspect drawer; Edit setup sheet
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-ta-replay-20260730-062156-531636-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: TA-018; TA-019; TA-task-create-async-activation
---

Introduced by the opendesign tasks redesign (docs/design/opendesign/tasks/task-detail.html, implemented 2026-07-21). Visual contract evidence: .compozy/tasks/os-shell/evidence/visual/opendesign-redesigns/VC-T1/.

QA impact 2026-09-17 (issue #653): the Overview tab renders the task description through
`DescriptionCard`, which now takes the reading-tier prose ladder (H1 22 px, H2 18 px, H3 16 px), semibold
emphasis, accent-strong underlined links, and framed tables inside its bordered card. The 3-tab IA and the
command-state head are untouched. Not inspected in Storybook and not walked: confirm a description with
H1/H2 headings and a table still sits comfortably in the card with no horizontal overflow.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

qa-impact: 2026-09-30 shell rail v2. The window head is 48px. Already untested; expectation updated.

qa-impact: 2026-09-30 shell rail v2 (surf-tasks). The head status is now the canonical state glyph beside the literal status word (no tinted pill), an approval waiting on the operator reads in the needs-you accent (pill and band), priority is a neutral lightness ramp (no signal hue), a failed run's error sits on its own line above the retry guidance, and a suspended window keeps waiting instead of reporting "Task not found". Verification walk (both themes): `.compozy/tasks/shell-rail/reports/surf-tasks-T3.md`.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
