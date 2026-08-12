---
id: TA-scheduled-session-restart-recovery
area: TA
title: Recover a scheduled session during daemon restart
persona: Bruno
journey: J-recover-scheduled-job-restart
expected: Restarting near a recurring job fire reaches a ready replacement daemon, preserves the registered job, and produces unique post-restart fire ids whose linked sessions remain readable across browser, HTTP, UDS, and CLI views.
entry_points: web /jobs; POST /api/settings/actions/restart; automation CLI and UDS reads
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: web/e2e/__tests__/jobs-hardening.spec.ts; internal/store/globaldb/global_db_goal_binding_integration_test.go; .tmp/playwright/test-results/__tests__-jobs-hardening-s-cf26d-does-not-duplicate-fire-ids/compozy-artifacts/manifest.json
last_report: docs/qa/reports/2026-08-12-post-merge-regression.md
overlaps: TA-schedule-catchup-overlap
---

Added after the post-merge Web lane exposed a canceled session start whose terminal metadata could
not finish binding the identityless provisional catalog row. The production recovery fence and the
same public restart journey passed on 2026-08-12.
