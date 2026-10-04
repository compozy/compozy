# BUG-20261002-task-action-profile-scope: Task control commands lose the selected profile

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-operate-daemon-schema, act on the overview's pending task approval
- **Scenarios:** RT-observe-overview-cli; TA-010
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
