# BUG-20261004-task-session-wrong-profile: Starting a task creates its session in another profile

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-drain-scheduler, inspect admitted work before maintenance
- **Scenarios:** TA-048
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Dora starts a maintenance task in resume-editorial. The run reports a session, but opening that
session under the task's profile returns session not found. The aggregate session catalog locates
it in default instead. The operator cannot inspect the admitted work from its owning profile.

## Reproduction

- **Charter:** CH-scheduler-drain-recovery · **Tour:** Interrupt Tour
- **Environment:** desktop, 1512 × 862, local Wi-Fi, en-US; real isolated daemon and Codex provider

1. Create a workspace task with --profile resume-editorial and start it with the same selection.
2. Start its queued run through task run start with --profile resume-editorial.
3. Read the returned session through session status/history with that same profile: both return 69.
4. GET its session detail over UDS with profile=resume-editorial: 404.
5. List sessions with --all --all-profiles --all-workspaces and the exact session ID query.
6. The catalog identifies default as owner; status/history under default succeed.

**Expected:** A dedicated task session retains its task's stable profile identity.
**Actual:** The task and run belong to resume-editorial, while the session belongs to default.

## Evidence

Receipts in docs/qa/evidence/2026-10-02-untested/:
- scheduler-drain-dora-work-start.json and scheduler-drain-dora-run-current.json
- scheduler-drain-dora-session-status.json and scheduler-drain-dora-session-history.json
- scheduler-drain-dora-session-owner-catalog.json
- scheduler-drain-dora-session-actual-owner-status.json and actual-owner-history.json
- scheduler-drain-dora-active-ended.json

The retained identities are task-62151111a019dfeb, run-d26da17fae537929 and
sess-aaa7019abbe26619. Drain timeout preserves the running run. Dora restores dispatch through
a fresh CLI and confirms paused=false through UDS without canceling or nudging the work.

## Fix

- **Root cause:** taskSessionBridge.StartTaskSession omits ProfileID from session.CreateOpts.
  The session manager applies its default-profile behavior to that empty field.
  The automatic role and starvation paths also omit the same identity, and role-session reuse
  does not compare profile ownership. Existing canonical cases reproduce all three omissions
  and the reuse error before repair.
- **Fix commit:** pending
- **Regression test:** extend the existing dedicated-system-session cases in
  internal/daemon/task_runtime_test.go; the daemon bridge owns task-to-session profile propagation.

## Verification

Pending the owning red/green check and a fresh original-persona replay.

The focused race checks now pass, including pool/starvation creation and foreign-profile reuse.
In the fresh Dora replay, both direct and automatic task sessions belong to resume-editorial;
owner-profile session detail succeeds and a default-profile read refuses the direct session.
The real pool worker claims and completes natively, delivering a readable workspace artifact.
Evidence: task-start-owner-final-focused.json and scheduler-drain-dora-replay-ended.json.
The original symptom is verified; delivery gate and commit are still pending.
