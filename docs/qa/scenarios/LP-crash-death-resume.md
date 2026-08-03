---
id: LP-crash-death-resume
area: LP
title: Resume one Loop node after its managed session dies
persona: Bruno
journey: J-recover-loop-node-failure
expected: Confirmed managed-session death reserves exactly one checkpoint-carrying continuation with a new epoch, cancel wins any race, parked nodes never resume, progress resets the death streak, and three consecutive deaths raise resume_exhausted attention.
entry_points: `compozy loop status --run-id <run-id> -o json`; Loop node inventory and event history over CLI/HTTP/UDS; daemon restart
qa_status: blocked-verify
bug_ids: BUG-20260803-loop-boot-active-coordinator-lease
fix_status: fixed
retest_status: pass
fix_commits: Task 13 checkpoint
evidence: internal/daemon/loop_node_lifecycle_e2e_integration_test.go; public QA has no managed provider-session death injector
last_report: docs/qa/reports/2026-08-03-loop-node-lifecycle.md
overlaps: LP-days-long-node-no-clock; TA-action-run-liveness
---

acceptance-walk: Kill a checkpointing node's managed session at progress and no-progress boundaries, race one death with cancel, and repeat three deaths without progress. Confirm one fenced continuation per confirmed death, cancel wins, parked work never resumes, progress resets the streak, and the final resume_exhausted attention is identical across CLI and HTTP fresh reads.
