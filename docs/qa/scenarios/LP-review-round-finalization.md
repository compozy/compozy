---
id: LP-review-round-finalization
area: LP
title: Finalize a review round only after complete triage
persona: Bruno
journey: J-08
expected: Finalization changes only fixed valid issues to resolved, preserves pending, invalid, unresolved, and blocked issue files, and returns exact resolved, invalid, and pending counts.
entry_points: workspace .compozy/tasks/<task>/reviews-NNN; loop run status; ext__spec_cycle__finalize_review_round
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-09-04-pr-543-review-findings.md
last_report: docs/qa/reports/2026-09-04-pr-543-review-findings.md
overlaps: LP-029;LP-review-artifact-inspection
---

Task07 2026-07-27: added for round-finalization behavior; flag only, not retested.

PR #543 2026-09-04: verification is blocked because this workstream explicitly prohibits local E2E and scenario execution. The canonical extension contract tests cover the status transitions, but they are engineering evidence rather than a persona walk.
