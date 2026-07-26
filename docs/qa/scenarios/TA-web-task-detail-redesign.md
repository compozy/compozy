---
id: TA-web-task-detail-redesign
area: TA
title: Task detail 3-tab IA with command-state head
persona: Bruno
journey: J-complete-task-tree
expected: Task detail renders Overview/Runs/Activity tabs with the 44px drill-in head (back, Tasks / <task> trail, status pill, one primary action from the §6 command machine — recover > publish > approve > resume > open run > retry > start — plus overflow verbs), an outcome/now strip matching the task state, subtasks with stacked progress, the 320px properties rail (priority + auto-enqueue editable; owner read-only), and the Inspect drawer (Diagnostics/Stream/Bridges/Raw). Nullable metrics render "—"; no set-status or delete-run control exists anywhere.
entry_points: web /tasks/:id (Overview, Runs, Activity tabs); Inspect drawer; Edit setup sheet
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-018; TA-019; TA-task-create-async-activation
---

Introduced by the opendesign tasks redesign (docs/design/opendesign/tasks/task-detail.html, implemented 2026-07-21). Visual contract evidence: .compozy/tasks/os-shell/evidence/visual/opendesign-redesigns/VC-T1/.
