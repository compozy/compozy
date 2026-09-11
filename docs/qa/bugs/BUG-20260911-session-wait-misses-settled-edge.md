# BUG-20260911-session-wait-misses-settled-edge: An open waiter misses completion in a visible session

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno; Ada
- **Journey Step:** J-26 observe a paused Goal boundary; J-15 wait for session completion
- **Scenarios:** RT-session-wait-state
- **Found:** 2026-09-11
- **Report:** docs/qa/reports/2026-09-10-qa-execution-unblock.md

## Reproduction

With real Cursor session sess-a98e031bb9aae7d6 visible in Web, register an idle wait while its Goal prompt is running. The prompt settles and the public status becomes idle, but the open wait returns timeout after90s with revision0. A fresh wait returns state-reached/idle immediately with revision1. Evidence: goal-extension-boundary-wait.json, goal-extension-session-at-exhaustion.json, goal-extension-idle-recheck.json. The earlier CLI transport fix correctly preserves the90s wait, exposing this separate missing transition.

## Cause and bounded fix

Prompt finalization clears runtime activity before publishing the settled attention commit. The publisher reconstructs both before and after badges using that already-cleared activity. A visible prompt therefore appears idle-to-idle and emits no edge. Preserve the full pre-finalization snapshot and publish the single canonical lifecycle attention transition after activity cleanup and the settled commit. Presence renewals and interaction commits retain their existing publisher.

## Cross-surface impact

CLI/HTTP/UDS/native waits, session catalog attention events, attention hooks and spawn wake observers receive the same committed final badge transition. No new event shape, native tool ID, config key, permission rule, workspace binding or migration is introduced. Web presence continues to mark visible completion seen; receipt/unread ownership is unchanged. Official skill already promises exact-state waits and committed attention transitions. Canonical race regression and real public retakes passed; make gate passed all affected lanes.

## Adjacent start transition

The corrected settled edge reached all three live CLI/HTTP/UDS waiters after33s. The same run exposed the paired start omission: an already registered running wait expired at45s while the public session and screenshot showed active prompting. Evidence: wait-edge-running.json, wait-edge-retake-current.json and wait-edge-retake-active.png. The runtime activity start boundary must publish its canonical transition as well. This is the same missing activity-transition ownership, so it is tracked here rather than a duplicate bug.

## Retest

The existing TestWaitForBadgeMatchesSnapshotsAndEdges suite owns the invariant: a registered waiter and canonical attention observers receive exactly one prompt start and one settled transition, with visible completion idle and unseen completion done. Both cases failed before their respective fixes and pass afterward with race detection; adjacent presence, hook and prompt ownership suites pass.

Real Cursor Grok 4.6 High Fast sessions confirmed CLI/HTTP/UDS settled waits after about 33 seconds, then all three pre-registered running waits after about 24 seconds. The final catalog stream contains exactly idle → running → idle; the Goal completed with one approved turn and retained Done/settled after Web reload. Origin stop is confirmed. The final run reuses the immediately preceding settled-wait acceptance because the subsequent start publication does not change finalization. Evidence: wait-both-edges-proof.json and its linked raw captures. No claim of a fresh final-run idle registration or a new replay of every historical wait branch.

Fix commit: 6149b72cf. All affected make gate lanes and explicit no-stash pre-commit checks passed.
