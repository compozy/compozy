---
id: TA-web-jobs-zero-inventory-suggestions
area: TA
title: Start a first job from live zero-inventory suggestions
persona: Bruno
journey: J-start-from-empty-catalogs
expected: The empty Jobs catalog explains the object, composes live workspace suggestions after the intro, keeps Create job and Dismiss server-backed, never renders the suggestions panel on a populated catalog, and preserves filtered, loading, error, Global-scope, and no-workspace behavior.
entry_points: web /jobs
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-15-pr409-empty-states/jobs-empty.png; docs/qa/evidence/2026-08-15-pr409-empty-states/jobs-suggestion-expanded-pointer.png; docs/qa/evidence/2026-08-15-pr409-empty-states/jobs-suggestion-expanded-keyboard.png; docs/qa/evidence/2026-08-15-pr409-empty-states/jobs-populated-after-refresh.png; docs/qa/evidence/2026-08-15-pr409-empty-states/jobs-filtered-empty.png; docs/qa/evidence/2026-08-15-pr409-empty-states/jobs-global-no-suggestions.png; docs/qa/evidence/2026-08-15-pr409-empty-states/jobs-loading.png; docs/qa/evidence/2026-08-15-pr409-empty-states/jobs-error.png
last_report: docs/qa/reports/2026-08-15-pr409-empty-states.md
overlaps: TA-automation-suggestions
---

Introduced by the OpenDesign empty-states redesign (`docs/design/opendesign/empty-states/`, Jobs reference). Accept/dismiss semantics remain owned by `TA-automation-suggestions`; this scenario owns the zero-inventory composition and first-use interaction.

## 2026-08-14 walk

Partial historical evidence only. The run covered keyboard disclosure, accept/dismiss, refresh, filtered-empty recovery, and compact layout, but did not independently prove pointer disclosure, loading, error, Global-scope, no-workspace, or populated precedence. A fresh isolated walk owns the final verdict.

## 2026-08-15 walk

Pass. A fresh isolated workspace covered pointer and keyboard disclosure, Dismiss and Create job through the live daemon, persistence after refresh, filtered-empty recovery, populated precedence, Global-scope suppression, and deliberate loading/error behavior. Public API and structured CLI reads independently confirmed one accepted Job and two remaining pending suggestions.
