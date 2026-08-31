---
id: LP-live-pause-repair-resume
area: LP
title: Pause one Loop node, repair it, and resume safely
persona: Bruno
journey: J-recover-loop-node-failure
expected: A manual or rule-driven pause parks only the selected node at a safe boundary with provenance, excludes it from scheduling and clocks, and each resume variant continues once with the requested attempt policy while healthy sibling work remains intact.
entry_points: `compozy loop node pause|resume`; HTTP/UDS node-control routes; native tools; Web run controls
qa_status: pass
bug_ids: BUG-20260803-cross-origin-coordinator-duplicate
fix_status: fixed
retest_status: pass
fix_commits: Task 13 checkpoint
evidence: looprun-97e; looprun-980; looprun-2e361; looprun-45fa; internal/store/globaldb/global_db_task_claim_test.go
last_report: docs/qa/reports/2026-08-31-issue-500-forced-loop-cancel.md
overlaps: LP-forced-cancel-owned-sessions; LP-quarantine-diagnose-requeue
---

acceptance-walk: Pause one live node at a safe boundary while a sibling continues, verify provenance and clock exclusion, then exercise plain, reset-attempts, and immediate resume variants on separate runs. Compare Web controls with structured CLI and HTTP state after refresh and confirm each accepted resume continues once with the requested attempt policy.

QA canary 2026-08-31: re-walk Pause and Resume beside the forced Cancel hard cut to catch shared
lifecycle-control regressions.
