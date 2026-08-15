---
id: TA-web-tasks-zero-inventory-templates
area: TA
title: Start a first task from the zero-inventory catalog
persona: Bruno
journey: J-start-from-empty-catalogs
expected: The empty Tasks catalog explains the object, offers Blank task and four collapsed templates, and opens the existing editor with the chosen real template while filtered, loading, and error states stay distinct.
entry_points: web /tasks
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-15-pr409-empty-states/tasks-empty.png; docs/qa/evidence/2026-08-15-pr409-empty-states/tasks-template-expanded-pointer.png; docs/qa/evidence/2026-08-15-pr409-empty-states/tasks-template-expanded-keyboard.png; docs/qa/evidence/2026-08-15-pr409-empty-states/tasks-template-editor.png; docs/qa/evidence/2026-08-15-pr409-empty-states/tasks-filtered-empty.png; docs/qa/evidence/2026-08-15-pr409-empty-states/tasks-triggers-loading.png; docs/qa/evidence/2026-08-15-pr409-empty-states/tasks-triggers-error.png
last_report: docs/qa/reports/2026-08-15-pr409-empty-states.md
overlaps: MS-web-task-editor-window-modal; TA-task-template-preserves-draft
---

Introduced by the OpenDesign empty-states redesign (`docs/design/opendesign/empty-states/`, Tasks reference). Visual contract evidence lands under `.compozy/tasks/empty-states/evidence/visual/`.

## 2026-08-14 walk

Partial historical evidence only. The run covered keyboard expansion, editor cancel/refresh, and compact layout, but did not independently prove pointer disclosure, filtered, loading, or error precedence. A fresh isolated walk owns the final verdict.

## 2026-08-15 walk

Pass. The fresh isolated walk covered the four-template zero-inventory catalog, pointer and keyboard disclosure, the real template-backed editor, cancel and refresh, filtered-empty precedence, and deliberate loading/error behavior.
