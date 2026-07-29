---
id: LP-agent-authored-review-run
area: LP
title: Start an agent-authored review run without a provider
persona: Bruno
journey: J-08
expected: Starting review-and-fix with task_name and optional reviewer, fixer, and auto_commit inputs launches the reviewer directly and exposes the same provider-free run through Web, CLI, HTTP, UDS, and native-tool surfaces.
entry_points: web review-and-fix run form; compozy loop run --name review-and-fix; POST /loops/review-and-fix/run; compozy__loop_run
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/review-and-fix-e2e.log; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-integration-rerun.log
last_report: docs/qa/reports/2026-07-27-devtool-oss-launch.md
overlaps: LP-029;TA-081
---

Task07 2026-07-27: added for the agent-authored review hard cut; flag only, not retested.
