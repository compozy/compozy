---
id: TA-web-tasks-zero-inventory-templates
area: TA
title: Start a first task from the zero-inventory catalog
persona: Bruno
journey: J-start-from-empty-catalogs
expected: The empty Tasks catalog explains the object, offers Blank task and four collapsed templates, and opens the existing editor with the chosen real template while filtered, loading, and error states stay distinct.
entry_points: web /tasks
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/empty-states/evidence/visual/zero-inventory/VC-01; .compozy/tasks/empty-states/evidence/visual/zero-inventory/VC-02; docs/qa/evidence/2026-08-14-empty-states/CH-empty-catalog-first-use-tasks-workspace.png; docs/qa/evidence/2026-08-14-empty-states/CH-empty-catalog-first-use-tasks-expanded.png; docs/qa/evidence/2026-08-14-empty-states/CH-empty-catalog-first-use-tasks-editor.png; docs/qa/evidence/2026-08-14-empty-states/CH-empty-catalog-first-use-compact-tasks.png
last_report: docs/qa/reports/2026-08-14-empty-states.md
overlaps: MS-web-task-editor-window-modal; TA-task-template-preserves-draft
---

Introduced by the OpenDesign empty-states redesign (`docs/design/opendesign/empty-states/`, Tasks reference). Visual contract evidence lands under `.compozy/tasks/empty-states/evidence/visual/`.

## 2026-08-14 walk

Passed in an isolated live workspace. Keyboard expansion exposed the One-shot details, Use template opened the real task editor in `empty-states`, cancel plus refresh kept the task count at zero, and the 768 px layout had no horizontal overflow.
