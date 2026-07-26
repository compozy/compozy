---
id: GL-use-response-as-goal
area: GL
title: Turn an assistant response into a Goal
persona: Lea
journey: J-26
expected: Use as Goal gives immediate feedback and prefills or starts one session-scoped Goal from the selected assistant response, with no hidden side effect when cancelled.
entry_points: web assistant response action; web session composer
qa_status: pass
bug_ids: BUG-20260713-use-as-goal-inert
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/goal-use-as-goal-live-fixed.dom.txt;/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/goal-use-as-goal-protects-draft.dom.txt
last_report: docs/qa/reports/2026-07-13-automation-features.md
overlaps: GL-001;GL-013
---

Exercise pointer activation on a completed real-provider response, verify exact focused prefill plus protected/discardable drafts and zero hidden submission, and pair the live result with the production keyboard-interaction contract.
