---
id: TA-web-jobs-zero-inventory-suggestions
area: TA
title: Start a first job from live zero-inventory suggestions
persona: Bruno
journey: J-start-from-empty-catalogs
expected: The empty Jobs catalog explains the object, composes live workspace suggestions after the intro, keeps Create job and Dismiss server-backed, never renders the suggestions panel on a populated catalog, and preserves filtered, loading, error, Global-scope, and no-workspace behavior.
entry_points: web /jobs
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/empty-states/evidence/visual/zero-inventory/VC-03; .compozy/tasks/empty-states/evidence/visual/zero-inventory/VC-04; docs/qa/evidence/2026-08-14-empty-states/CH-empty-catalog-first-use-jobs-entry.png; docs/qa/evidence/2026-08-14-empty-states/CH-empty-catalog-first-use-jobs-expanded.png; docs/qa/evidence/2026-08-14-empty-states/CH-empty-catalog-first-use-jobs-refresh.png; docs/qa/evidence/2026-08-14-empty-states/CH-empty-catalog-first-use-jobs-filtered.png; docs/qa/evidence/2026-08-14-empty-states/CH-empty-catalog-first-use-compact-jobs.png
last_report: docs/qa/reports/2026-08-14-empty-states.md
overlaps: TA-automation-suggestions
---

Introduced by the OpenDesign empty-states redesign (`docs/design/opendesign/empty-states/`, Jobs reference). Accept/dismiss semantics remain owned by `TA-automation-suggestions`; this scenario owns the zero-inventory composition and first-use interaction.

## 2026-08-14 walk

Passed against the live daemon. A keyboard-expanded suggestion disclosed server data, Create job persisted the accepted job, Dismiss stayed resolved after refresh, public API reads confirmed the workspace-scoped records, filtered-empty recovered through Clear filters, and the 768 px layout had no horizontal overflow.
