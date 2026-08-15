---
id: TA-web-triggers-zero-inventory-intro
area: TA
title: Start a first trigger without invented suggestions
persona: Bruno
journey: J-start-from-empty-catalogs
expected: The empty Triggers catalog explains the object, opens the existing create flow, renders no suggestion panel the runtime cannot support, and preserves filtered, loading, error, and populated precedence.
entry_points: web /triggers
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/empty-states/evidence/visual/zero-inventory/VC-05; .compozy/tasks/empty-states/evidence/visual/zero-inventory/VC-06; docs/qa/evidence/2026-08-14-empty-states/CH-empty-catalog-first-use-triggers-entry.png; docs/qa/evidence/2026-08-14-empty-states/CH-empty-catalog-first-use-triggers-editor.png; docs/qa/evidence/2026-08-14-empty-states/CH-empty-catalog-first-use-triggers-filtered.png; docs/qa/evidence/2026-08-14-empty-states/CH-empty-catalog-first-use-compact-triggers.png
last_report: docs/qa/reports/2026-08-14-empty-states.md
overlaps:
---

Introduced by the OpenDesign empty-states redesign (`docs/design/opendesign/empty-states/`, Triggers reference). The missing suggestion panel is a runtime-owned authorized difference, not a UI gap to fill.

## 2026-08-14 walk

Passed in the isolated workspace. The empty catalog showed only the supported create action, the real trigger editor opened and canceled without persistence, filtered-empty recovered through Clear filters, no suggestion panel appeared, and the 768 px layout had no horizontal overflow.
