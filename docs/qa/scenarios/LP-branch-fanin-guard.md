---
id: LP-branch-fanin-guard
area: LP
title: Merge alternate branch paths and guard required conditions
persona: Bruno
journey: J-complete-partial-loop
expected: A shared join waits for all dependencies to settle and runs when one incoming path is active even if another is branch_skipped; an explicit AND branch that is the protected action's sole predecessor skips that action unless every required condition holds.
entry_points: compozy loop create|run|status; GET /api/workspaces/:workspace_id/loop-runs/:run_id; /docs/loops/dsl-reference; skills/compozy/references/loops.md
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-10-18-issue-777-branch-fanin.md
last_report: docs/qa/reports/2026-10-18-issue-777-branch-fanin.md
overlaps: LP-exclusive-route-history
---

acceptance-walk: Publish the fork → true/false branches → transforms → join graph from issue #777.
Run it and confirm false_path has branch_skipped evidence while true_path and join execute. Then
publish the documented guarded-delivery example. With security_ok=true and approval_ok=false,
publish must be branch_skipped without a task_run_id. With both true, publish must execute and
return published=true. Compare each terminal generation through CLI/UDS and an independent HTTP
read. A succeeded status with a skip marker must never be interpreted as proof that work executed.

2026-10-18: All three mechanical runs settled done on an isolated beta.35 daemon. CLI/UDS and HTTP
generation outputs matched exactly. The existing alternative-path contract is preserved; the docs
now explain its dependency barrier and the explicit all-conditions guard.
