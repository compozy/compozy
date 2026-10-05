# BUG-20261005-loop-filter-hides-existing-runs: A filtered Runs list claims the profile has no runs

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Marina
- **Journey Step:** J-03, narrow the Runs queue and recover from no matches
- **Scenarios:** LP-008
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Choosing an origin with no matching runs displays the profile's first-run message and
offers Browse loops, although the unfiltered roster contains 35 runs. The empty state
does not explain that a filter is hiding existing history or offer to clear it.

## Reproduction

CH-002, Interrupt Tour, Marina's 430x932 touch viewport at DPR 3, 4G, en-US.
Open the Loop catalog, choose Runs, then Add filter → Origin → Session. Wait for the
successful filtered response. The screen says “No runs in resume-editorial yet” and
offers Browse loops instead of a filtered-empty recovery action.

## Evidence

`loops-human-review-origin-settled.json` records the settled UI. The screenshot
`loops-human-review-origin-empty-before.png` and receipt
`loops-human-review-origin-empty-close.json` confirm the visible Origin chip and false
first-run message. All evidence is under `docs/qa/evidence/2026-10-02-untested/`.
The twenty-frame `loops-human-review-fixed` recording is closed before repair.

## Fix

- **Root cause:** The roster's empty model, scope title, and recovery action consider only
  the client-side outcome filter. Origin and session-id filters already reach the server
  but their presence is lost before empty-state projection.
- **Scope:** Carry active-filter presence through the existing roster model and clear all
  roster filters through their existing setters. No API, storage, or shared primitive change.
- **Regression test:** The existing `loop-runs-location.test.tsx` route-composition suite
  owns filtered-empty recovery for origin, session id, and combined outcome filtering.
- **Fix commit:** acbeed2ec31a6d7c2f97fc82d00e0271d904ad70.
- **Retest:** passed in Marina's 430x932 touch / 4G walk. Clear filter restores all 35
  rows after either origin-only or exact-session empty results and survives reload.
  Independent UDS returns 35 unfiltered runs and zero exact-session matches. Evidence:
  loops-filter-origin-replay-complete.json, loops-filter-clear-reload.json,
  loops-session-filter-marina-verified.json and loops-session-filter-final-*-uds.json.
  Required make gate passed; fix commit: acbeed2ec31a6d7c2f97fc82d00e0271d904ad70.

Delivery closure: make gate passed on the frozen tree (loops-human-review-delivery-gate-v6.json). The commit hook changed no file content; loops-human-review-committed-head.json records the checked hashes and commit.
