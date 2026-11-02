---
id: ET-worktree-hook-event-contract
area: ET
title: Gate worktree mutations while lifecycle observations stay fail-open
persona: Ada
journey: J-worktree-management
expected: Explicit pre_create and pre_remove denials block with the hook name and reason, hook execution errors fail open, adopted worktrees skip pre_create, observe-only consumers cannot block transitions, and every durable worktree event is ordered, workspace-attributed, correlated, and redacted on replay.
entry_points: worktree.pre_create|pre_remove hooks; worktree.created|adopted|removed observations; HTTP/UDS worktree mutations; per-worktree SSE; worktree catalog SSE; compozy hooks list|info|events|runs -o json
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/native-worktree-create.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/native-worktree-remove.json; internal/daemon/daemon_worktree_e2e_integration_test.go
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: RT-worktree-api-surface-parity; TA-task-per-run-worktree-isolation
---

QA impact: Task 01 adds the hook family and canonical event registry. The walk must cover manual and
per-run creation, removal, adoption, setup failure, creation cancellation, status refresh, safe
branch reclamation, and every exit-action terminal path, then reconnect and prove replay has no gap,
duplicate, foreign-workspace frame, or raw planted secret.

Quiet exit-plan read (2026-10-31): `GET .../worktrees/{id}/exit` refreshes and persists the cached
status without emitting `worktree.status_refreshed`; status refreshes such as
`GET .../status?refresh=true`, the CLI refresh, finished turns, and settled actions emit it. The walk must read the exit plan several times against a live
per-worktree stream and see no new frame, and a session page bound to a worktree must hold a steady
request rate instead of refetching the plan on its own frames.


2026-11-02 responsiveness follow-up (focused evidence; integrated replay pending): catalog
wake signals now share the existing 150 ms quiet window and 1 s maximum wait. During a burst,
verify that the catalog converges to the latest durable state, pending work flushes on catalog
error or connection disposal, failure text appears immediately, and foreign-workspace frames
remain inert. A per-worktree wake rereads its exact list/detail/status/exit scope without
refetching a sibling worktree. These changes affect consumer rereads; the durable event order,
hook decisions and replay cursor contract retain their existing owners.

Recorded engineering evidence is in
[the worktree/workspace performance report](../reports/2026-11-01-performance/worktree-workspace.md):

- Root Turbo with Bun 1.4.0 passed 28 tests across the existing catalog-stream, worktree-stream
  and worktree-exit hook suites (`.cache/performance/worktree-workspace/stream-after.log`).
  Real QueryObserver fixtures converged after 100 catalog frames with three query-function
  calls instead of 400; one detail wake made four calls instead of eight, with zero sibling
  calls. This proves cache reconciliation and request amplification, not browser latency.
- `TestServiceStatus`, `TestPullRequestService` and `TestServiceDiscovery` passed with `-race`
  in 2.131 s (`worktree-race.log`). The clean-status benchmark used a real temporary Git
  repository with 256 tracked files: five-run median 45.699 -> 16.169 ms/op; dirty/error and
  upstream facts remain owned by the canonical status suite (`status-after.log`).
- The root queued the existing `TestWorktreeLifecycleIntegration` cases for real status
  corruption/recovery, upstream divergence, dirty/unpushed refusal and external-commit evidence
  rereads under `-race`. Its receipt is pending at this note; no integration pass is inferred
  from the benchmark or unit/race suites.

Existing automated journey mapping remains
`internal/daemon/daemon_worktree_e2e_integration_test.go::TestDaemonWorktreeStreamsAndIsolationIT034IT038`
for HTTP/UDS isolation, ordered per-worktree replay and workspace-attributed catalog frames;
`web/e2e/__tests__/worktrees.spec.ts` owns E2E-006 creation/cancellation, the accepted-pending
creation case, and E2E-014 assisted-exit progress. Those daemon/browser journeys and current-head
CI have not been rerun by this worker. Historical verdict fields are unchanged; this note does
not record a new scenario pass.
