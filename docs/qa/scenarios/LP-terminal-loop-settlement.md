---
id: LP-terminal-loop-settlement
area: LP
title: Terminal Loop runs leave no live execution records
persona: Ada
journey: J-loop-terminal-recovery
expected: Natural completion, forced cancellation, crash recovery, and retention-orphan repair leave no claimable task runs; task timelines expose loop_run_terminal, reconciled_run_terminal, or run_missing as structured reasons.
entry_points: compozy daemon; compozy loop status <run-id>; compozy loop cancel <run-id>; compozy task timeline <task-id>; compozy task list --loop-run <run-id>; compozy config get loops.reconcile_interval -o json
qa_status: pass
bug_ids: BUG-20260821-sessionless-lease-recovery; BUG-20260821-coordinator-lease-exhausted
fix_status: fixed
retest_status: pass
fix_commits: 0a4fe2d; 69c2d74
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/settlement/boot-terminal-sweep.log; /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/settlement/terminal-loop-public-tasks.json; /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/settlement/boot-idempotent-second.log
last_report: docs/qa/reports/2026-08-31-issue-500-forced-loop-cancel.md
overlaps: LP-forced-cancel-owned-sessions; LP-loop-lifecycle-config-cli; LP-run-read-agent-journey; TA-parent-rollup-completion
---

Seed terminal and missing-run ownership shapes before daemon start. Verify the boot barrier removes
claim eligibility before recovery, a second boot emits no duplicate repair event, active Loop runs
remain unchanged, and the configured interval affects only later sweeps.

QA result 2026-08-21: the first strict boot failed closed on two reproduced coordinator lease
defects. After their focused fixes, the seeded terminal boundary settled three records and repaired
one orphan before readiness. Public task reads showed all three records canceled with
`reconciled_run_terminal`; the second boot settled zero records, emitted no duplicate repair event,
and left zero ready rows for the run. Public cancel and kill paths were also walked.

Issue 689 acceptance: with at least 12,000 settled task runs, hold another SQLite writer and run
an empty orphan sweep and provenance backfill. Both reads must finish without acquiring the writer.
Hold the writer across multiple action completion/failure write deadlines: release it and verify
one action invocation, a terminal task run, released lease, and settled generation output. Repeat
with contention during the action heartbeat and completed tool-event persistence. Empty heartbeat
retention must also remain read-only. See the owning
[contention verification report](../reports/2026-10-03-issue-689-sqlite-contention.md).
