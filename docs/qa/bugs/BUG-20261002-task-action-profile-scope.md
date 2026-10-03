# BUG-20261002-task-action-profile-scope: Task approval commands lose the selected profile

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-operate-daemon-schema, act on the overview's pending task approval
- **Scenarios:** RT-observe-overview-cli
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
- **Fix commit:** Pending.
- **Regression test:** The existing CLI profile command suite owns action selection at the
  client boundary. No daemon authorization or lookup fallback is needed.

## Verification

All four command regressions fail before the wrapper is registered and pass afterward with the race
detector. The rebuilt CLI approves a fresh Studio task and rejects the previously blocked proposal;
independent UDS reads persist both decisions. Explicit default-profile approval still refuses the
Studio task before mutation. Evidence: `task-action-profile-{red,green,build}.log` and
`task-profile-fixed-*.json`. Other task command families retain their own QA walks.
