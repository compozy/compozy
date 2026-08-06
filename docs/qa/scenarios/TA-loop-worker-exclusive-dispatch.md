---
id: TA-loop-worker-exclusive-dispatch
area: TA
title: Dispatch each Loop worker through exactly one runtime owner
persona: Bruno
journey: J-complete-task-tree
expected: A Loop action worker is claimed, prompted, and completed exactly once by the Loop executor without creating a coordinator or ordinary task-role activation, while an adjacent ordinary task-role run still activates normally.
entry_points: scheduler starvation recovery; Web agent Sessions; Web task run detail; Loop run detail
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits: working-tree
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-agent-ownership-r2-20260806-040706-936266-lab/qa-artifacts/qa/evidence/loop-owner-sessions.json;/Users/pedronauck/dev/qa-labs/compozy-loop-agent-ownership-r2-20260806-040706-936266-lab/qa-artifacts/qa/evidence/loop-cancel-events.sse
last_report: docs/qa/reports/2026-08-06-loop-agent-ownership.md
overlaps: TA-task-role-session-activation
---

The owning Loop run and worker task run must retain one session, one initial prompt, and one completion path. Coordinator recovery and task-role starvation recovery may observe the run but must not claim it.
