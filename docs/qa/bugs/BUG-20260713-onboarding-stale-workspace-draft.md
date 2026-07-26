# BUG-20260713-onboarding-stale-workspace-draft: A workspace restored from another daemon cannot be removed during onboarding

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Lea
- **Journey Step:** J-19 onboarding workspace selection, step 2
- **Scenarios:** RT-004
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** Fresh final replay bootstrap

## Summary

The in-app Browser retained an onboarding workspace selection created against an earlier daemon. A fresh isolated daemon did not contain that workspace ID, but step 2 restored the old folder into `Selected workspaces`. Clicking its unique Remove control sent the stale identity to the fresh daemon, rendered a `workspace not found` alert, and left the item selected.

## Reproduction

- **Charter:** CH-onboarding-stale-workspace · **Tour:** Back-Button Tour
- **Environment:** desktop / wifi-fast / en-US; fresh isolated daemon PID `47809` on port `56381`; in-app Browser; Cursor/Grok 4.5 selected in step 1.

1. Select a workspace during onboarding against daemon A without completing or clearing the browser-side onboarding draft.
2. Start a fresh daemon B with an empty catalog on the same Web origin and open first-run onboarding.
3. Continue to Workspaces and observe daemon A's folder under `Selected workspaces`.
4. Click the exact Remove control for that folder.

**Expected:** The stale selection is reconciled with daemon B or can be removed locally without requiring a successful lookup of an identity that daemon B never owned.
**Actual:** The wizard displays `workspace: lookup workspace "ws_06366aad69887872" by name fallback: workspace not found`, and the stale selected folder remains.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-final-replay-20260713-20260713-194432-535561-lab/qa-artifacts/qa/screenshots/rt-onboarding-stale-workspace-removal.dom.txt`

## Fix

- **Root cause:** The onboarding draft is browser-global for the origin, while `useWorkspaces()` is the authoritative catalog of the currently connected daemon. The removal hook promoted any persisted `workspaceId` to a current-daemon identity without membership validation. The fix now treats the draft ID as a hint only, resolves destructive identity against the settled current catalog by ID then path, removes locally when the settled catalog has no match, and preserves `undefined` as an unresolved third state that disables/no-ops Remove until authority is known.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** `web/src/systems/onboarding/hooks/__tests__/use-onboarding-workspaces.test.tsx` owns the boundary. The original red proved `DELETE ws_previous_daemon`; a second red proved that unresolved catalog data incorrectly exposed Remove and erased a valid draft. The final six-case suite covers unresolved, settled-empty, registered-success, registered-error, hydration, and non-overwrite behavior. Controller rerun passed 6/6; focused oxfmt, zero-warning oxlint, and diff-check passed. The worker also passed fresh Web typecheck and the full Web lane (3,407 tests); no peer review ran.

## Verification

- Browser retest passed in the original live tab without restarting the lab. The stale control disappeared with zero alerts, the current catalog rehydrated `pedronauck`, the real current workspace retained normal deletion, and onboarding completed after adding exactly `agh3` and `bench-ops`. The dashboard rendered with active workspace `agh3` and exactly two workspaces.
- Evidence: `/Users/pedronauck/dev/qa-labs/agh-automation-features-final-replay-20260713-20260713-194432-535561-lab/qa-artifacts/qa/screenshots/rt-onboarding-stale-workspace-removal-fixed.dom.txt`.
