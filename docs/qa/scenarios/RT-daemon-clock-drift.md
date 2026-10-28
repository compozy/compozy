---
id: RT-daemon-clock-drift
area: RT
title: Manage a live Linux daemon despite process start-time clock drift
persona: Ada
journey:
expected: A live daemon remains stoppable and restartable after Linux process start-time drift; duplicate start identifies its PID, bootstrap preserves its discovery record, and a reused PID without the home lock is never signaled.
entry_points: compozy daemon start; compozy daemon stop; POST /api/settings/actions/restart; compozy status -o json
qa_status: pass
bug_ids: https://github.com/compozy/compozy/issues/982
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-10-27-issue-982.md
last_report: docs/qa/reports/2026-10-27-issue-982.md
overlaps: APP-attach-running-daemon
---

Use an isolated home. Start the daemon and confirm its PID through status. A second
start must identify the running PID. Restart through Settings and confirm a new
healthy PID, then stop and independently confirm the listener is gone.

On WSL2, repeat after Windows host sleep/resume and after observing `ps` start-time
drift. Keep the existing discovery record intact: records from earlier versions
must work without manual edits or migrations. A live unrelated process referenced
by stale discovery data must not be signaled unless it owns this home's kernel lock.

Deterministic coverage belongs to `TestDaemonClockDrift`, the existing reused-PID
CLI suite, and `TestFileLockIdentity`. Linux container evidence does not establish
WSL2 host-suspend behavior.

2026-10-27: Linux container lifecycle and the deterministic 40-hour mismatch probe passed.
The pass applies to this bounded coverage; WSL2 host suspend/resume was not available.
