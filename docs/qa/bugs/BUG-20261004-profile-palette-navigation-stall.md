# BUG-20261004-profile-palette-navigation-stall: Opening profile creation can leave the desktop unresponsive

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-operate-profiles, submit a profile lifecycle command in an attached desktop
- **Scenarios:** ET-profile-remote-write-boundary; ET-profile-palette-view
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

CH-profile-lifecycle-plan-recovery, Interrupt Tour, isolated production Web at port 50727,
desktop Chrome 1512×862, en-US. Several existing task-owned tabs are attached to Studio Operations.
Choose Create profile in the palette, enter dispatch-palette, and submit. On the first attempt,
the active document stops answering accessibility and Runtime.evaluate requests. The browser
connection and root Target requests remain healthy. A second affected tab shows the consumed
name query while the originating tab retains flow=create&name=dispatch-palette.

Expected: the canonical dialog opens and the desktop remains responsive.
Actual: the first attempt cannot be observed or operated. Two owned affected tabs are closed after
capturing the failure; a fresh tab opens normally. One clean retry opens the dialog but loses the
name, recorded separately as BUG-20261004-profile-palette-drops-arguments.

## Evidence

Cycle receipts: profile-local-palette-argument-observed.json,
profile-local-palette-argument-final-state.json, profile-local-palette-browser-diagnostic.json,
profile-local-palette-hang-root-targets.json, profile-local-palette-hang-close-owned-tab.json,
and profile-local-palette-hang-recover-tab.json. Recording profile-local-palette-argument-ada
stops at four frames. Its last frame precedes the timeout and is not proof of an opened dialog.
Independent UDS still returns profile_not_found; no profile mutation occurred.

## Investigation

The stalled document and a second renderer consumed CPU while the real daemon stayed reachable.
The SQLite gate also ran concurrently; resource contention and competing route consumption remain
hypotheses, not established causes. Source edits start only after both persona attempts end.
The successful clean retry does not erase the initial failure. Reproduction with controlled
attached-client count and the corrected argument handoff remains required.

## Fix

Pending root-cause proof.

## Verification

Pending.
