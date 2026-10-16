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

Issue #739 regression slice:

- Hold the SQLite writer during the exact claim attempt for a non-reviewed `run-agent` action.
  The first bounded claim must fail without executing the action; after writer release, the
  daemon retries, observes the durable claim, executes once and commits node success.
- A bound worker's release and block-and-release requests must return permission denied,
  preserve the daemon claim and allow daemon completion to publish node success.
- Force-fail a queued and a claimed Loop worker. The node must become failed in the same
  transaction, and the terminal hook must wake the coordinator to apply existing retry policy.
- Successful session-backed task hooks carry the owning profile. Coordinator/mechanical
  terminals without a session must not report incomplete terminal-session identity.

Owning suites: `TestGlobalDBRunLeaseTerminalShouldRecordLoopNodeProgress`,
`TestLoopGoalManagedRuntimeIntegration/Should_wait_for_a_durable_claim`,
`TestTaskRunHookContextCarriesLoopFilterKeys`, and
`TestTerminalRunLifecycleObserverShouldReleaseCurrentSessionGeneration`.
Evidence and validation limits: `docs/qa/reports/2026-10-16-issue-739-loop-claim.md`.
