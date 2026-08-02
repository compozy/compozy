---
id: LP-crash-death-resume
area: LP
title: Resume one Loop node after its managed session dies
persona: Ada
journey: J-resume-dead-loop-node
expected: Confirmed managed-session death reserves exactly one checkpoint-carrying continuation with a new epoch, cancel wins any race, parked nodes never resume, progress resets the death streak, and three consecutive deaths raise resume_exhausted attention.
entry_points: `compozy loop runs show <run-id> -o json`; Loop node inventory and event history over CLI/HTTP/UDS; daemon restart
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-days-long-node-no-clock; TA-action-run-liveness
---

QA impact 2026-08-02: Task 05 implements the atomic `ResumeDeadNode` authority and deterministic
continuation binding. A real-user chaos walk is blocked until Task 07 exposes the node lifecycle
projection and Task 13 provisions the isolated crash/restart lab.
