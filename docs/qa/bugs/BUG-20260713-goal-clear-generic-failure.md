# BUG-20260713-goal-clear-generic-failure: Clearing an active Goal hides its revocation cause

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-26, clear a Goal while its continuation is in flight
- **Scenarios:** GL-019
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** CH-047 live control-boundary replay

## Summary

Clear correctly revokes the active session Goal and records `goal_control_revoked_in_flight` on the ambiguous turn, but the historical Loop Run settles as generic `Failed` with only “The action failed before producing an output.” The operator cannot tell that this was their requested control action rather than an infrastructure failure, even though the durable turn already owns the truthful cause.

## Reproduction

1. In `CH-047` / Interrupt Tour, start Goal `looprun-1667f72b7cdb7128` from a live Cursor/Grok session.
2. Allow the first turn to end without a Goal report so one continuation starts.
3. Click `Clear goal` while that continuation is in flight.
4. Open the historical Run and compare its top-level failure alert with the Goal turn timeline.

**Expected:** Clear settles exactly once, cancels the in-flight work, and projects the durable control-revocation cause plus actionable recovery without implying an unexpected infrastructure failure.
**Actual:** The session Goal disappears and the turn records `goal_control_revoked_in_flight`, but the Run-level alert is a generic action failure.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/goal-active-clear-canceled-turn.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/goal-active-clear-run-failed-generic.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/goal-judge-clear-residual.dom.txt`
- Run `looprun-1667f72b7cdb7128`, turn 2 Ambiguous, cause `goal_control_revoked_in_flight`.

## Fix

- **Root cause:** Clear committed the durable revocation before canceling the active judge, but the Goal executor returned the canceled judge context before re-reading that newer checkpoint. The shared Loop mapper therefore received plain `context canceled` instead of the existing safe Goal failure provider and reduced the Run to `loop_action_failed`. The executor error boundary now promotes the durable revocation only when the checkpoint is terminal, its `control_epoch` advanced, and its cause is `goal_control_revoked_in_flight`.
- **Fix commit:** uncommitted QA remediation batch
- **Regression test:** The canonical Goal executor, action-failure mapper, GlobalDB concurrent-replay integration, and daemon Goal E2E prove one `goal_control_revoked_in_flight` projection, zero late turns/judges, eight fenced replays, and stable event/turn counts.

## Verification

- Automated production integration passes under race with the typed cause and recovery plus exactly-once replay fencing.
- The active-judge lifecycle residual passed in a same-persona live replay: Clear returned HTTP 200 only after a 5.138-second cancel/join, Goal read returned `null`, judge `sess-3e07f85d0d2ac987` was durably `stopped/user_canceled`, active system-session count was zero, and Run `looprun-d8466636e525f1e5` retained exactly one generation with no late successor or duplicate prompt.
- Deterministic red reproduced the authoritative ordering: after the fake store committed revocation and the judge context was canceled, `Execute` returned only `context canceled`. The green regression requires the typed safe failure while retaining the committed checkpoint as authority.
- Final real-provider replay used session `sess-0c15074cd1431b4c`, Goal Run `looprun-0d9a6cc9afa3e92e`, and active judge `sess-4afdada5589b5fed`. The same `browser-use` process observed the judge as active, captured the control, and clicked Clear without a second harness startup.
- The Goal disappeared, the judge settled `stopped/user_canceled`, active system-session count returned to zero, and the Run retained generation 1 with no successor. Run detail rendered `goal_control_revoked_in_flight` as “Goal control revoked the in-flight turn.” with recovery “Start a new Goal to continue the objective.”
- Evidence: `/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/network/catalog-global-goal-acceptance.json`, `qa/screenshots/goal-clear-typed-third-during-judge-before.png`, `qa/screenshots/goal-clear-typed-third-after.png`, and `qa/screenshots/goal-clear-typed-run-detail.png` in the same lab.

## Intermediate re-find (2026-07-13; superseded by the final replay above)

Clear was invoked while the second live judge was active for `looprun-e6830bc6fd4a086f`. The button remained Loading, Pause stayed disabled, and turn 3 was admitted and dispatched after the Clear request. That continuation performed multiple Goal/Loop tool calls while the control remained pending. Even after `Stop generation` made the sessions idle/stopped, the connected Goal chip stayed active with Clear still Loading; reconnecting the permalink timed out. The typed historical failure projection cannot be accepted because the earlier control invariant is violated: Clear did not fence the successor turn or converge the live UI.

This intermediate failure reopened the bug and drove the later final real-provider replay documented in Verification. That later replay is the historical basis for `Status: verified`; the rebased judge-routing change resets GL-019 to `untested` until a new final-worktree replay.
