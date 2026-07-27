---
id: LP-review-round-finalization
area: LP
title: Finalize a review round only after complete triage
persona: Bruno
journey: J-08
expected: Finalization rejects any round with pending or malformed issue files without partial writes, then monotonically changes valid issues to resolved while preserving invalid triage and returning exact resolved, invalid, and pending counts.
entry_points: workspace .compozy/tasks/<task>/reviews-NNN; loop run status; ext__dev_cycle__finalize_review_round
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/review-and-fix-e2e.log; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-integration-rerun.log
last_report: docs/qa/reports/2026-07-27-devtool-oss-launch.md
overlaps: LP-029;LP-review-artifact-inspection
---

Task07 2026-07-27: added for round-finalization behavior; flag only, not retested.
