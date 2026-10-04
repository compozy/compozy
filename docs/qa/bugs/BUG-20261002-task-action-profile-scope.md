# BUG-20261002-task-action-profile-scope: Task control commands lose the selected profile

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-operate-daemon-schema, act on the overview's pending task approval
- **Scenarios:** RT-observe-overview-cli; TA-010; ET-profile-lifecycle-race-guards
- **Found:** 2026-10-02 · **Report:** docs/qa/reports/2026-10-02-untested.md
- **Origin:** Isolated primary QA lab

## Reproduction

During CH-untested-066-operate-daemon-schema-ada (Garbage Tour):

1. Create a workspace task with manual approval under profile `studio` through the public API.
2. Read it with `compozy task get <id> --profile studio -o json`; the task is present.
3. Read the overview with the same profile; it offers `approve`, `reject`, and `open`.
4. Run `compozy task approve <id> --profile studio -o json`, or reject a second pending task.
5. Both exit 69 with `task: task not found`. A second approval command reproduces the refusal.
6. POST the same approval over HTTP with `?profile=studio`; it succeeds with 201 and one queued run.

**Expected:** The task action uses the selected profile, matching the task and overview reads.
**Actual:** The action silently drops the profile and targets the daemon's default profile.

## Evidence

`docs/qa/evidence/2026-10-02-untested/overview-approval-open.json`,
`overview-approval-cli.json`, `overview-reject-action.json`, `overview-approve-action.json`,
`overview-approval-refusal-confirm.json`, and `overview-approve-http.json`.

## Fix

- **Root cause:** The shared publish/start/approve command constructor and the reject command
  omit the existing profile-selection wrapper. Their transport only receives selection from
  command context, so an accepted root flag never reaches the request.
- **Fix commit:** `8d1e73ab3`.
- **Regression test:** The existing CLI profile command suite owns action selection at the
  client boundary. No daemon authorization or lookup fallback is needed.

## Verification

All four command regressions fail before the wrapper is registered and pass afterward with the race
detector. The rebuilt CLI approves a fresh Studio task and rejects the previously blocked proposal;
independent UDS reads persist both decisions. Explicit default-profile approval still refuses the
Studio task before mutation. Evidence: `task-action-profile-{red,green,build}.log` and
`task-profile-fixed-*.json`. Other task command families retain their own QA walks.

## Reopened for adjacent control commands — 2026-10-04

The earlier publish/start/approve/reject repair remains verified. A fresh public pause command
under resume-editorial returns task not found while task get with the same ID/profile succeeds.
Evidence: extension-task-state-replay-pause-with-reason.json and
extension-task-state-replay-task-get.json. The same missing context wrapper remains on pause,
resume, cancel, block, unblock, blocks and recover. The existing CLI profile boundary suite
reproduces all seven omissions in extension-task-full-state-and-controls-red.json. Apply the
established mutation wrapper, and the existing single-profile read wrapper for blocks; leave
authorization and daemon task lookup unchanged. Fresh public replay is required for this extension.

The same replay observes an unrelated CLI description error: source policy asks approval for
a read-only command, but its pending message says destructive. Use the generic approval message;
no prose-only regression test is added.

## Control-family verification — 2026-10-04

The seven existing control commands now establish their selected profile before transport.
The profile regression suite passes with the race detector. In the rebuilt app, a real owned
task supports CLI pause/resume/block/blocks/unblock/recover/cancel; independent HTTP/UDS reads
persist every mutation. A foreign-profile resume still returns not found without effect. Web
reload/deep-link observations confirm paused and canceled states. Evidence:
extension-task-full-state-replay-ended-summary.json. This closes the adjacent control-family
recurrence without broadening the original overview scenario's verdict.

## Reopened for run and related operator actions — 2026-10-04

Dora creates a ready task under publishing-queue, then task run enqueue with that explicit profile
returns task not found. The concurrency session ends before source diagnosis. Its retained
profile-work-race-enqueue.json is the public reproduction; scheduler resume restores baseline.

The same missing command context wrapper remains in run enqueue/start/attach/complete/fail/cancel/
recover, force release/fail (single and bulk), retry, fan-out, task update/delete, dependencies, and
review request/submit. Nineteen transport-boundary cases in the existing profile_test.go fail
with an empty outgoing profile before repair (task-operator-profile-red.json). The fix registers
the existing mutation wrapper at those constructors; daemon routes, ownership validation and
session-bound identity remain authoritative. The separate task execution-profile payload flag
has its own pre-existing --profile naming collision and is outside this run-control repair.

The previous approval and pause/block/control-family replays remain valid for their unchanged
actions. This extension requires affected race checks and a fresh public run-control walk.

## Run-control verification — 2026-10-04

The existing profile suite proves all nineteen added entry points stamp the selected profile.
The fresh frozen-build Dora walk enqueues under publishing-queue, freezes/unfreezes the same run,
acquires it through an authenticated session, refuses a foreign force-fail, and persists owner
force-fail, retry and cancellation through independent HTTP/UDS reads. Task update, dependency
add/remove and deletion also succeed under the selected owner. Evidence:
task-operator-profile-green3.json, task-operator-profile-delivery-gate.json, and
profile-work-race-replay-ended.json. Test conventions pass with zero findings. The gate passes
codegen, zero-issue Go lint, affected race suites and cached Web evidence (693 files, 6,927 tests).
The new queue-owner projection and archive-race diagnostic findings have separate registry IDs.
