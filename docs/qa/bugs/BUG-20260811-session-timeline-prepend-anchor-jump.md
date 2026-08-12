# BUG-20260811-session-timeline-prepend-anchor-jump: Loading older history loses the visible message anchor

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Rafa
- **Journey Step:** J-14 page older transcript history
- **Scenarios:** RT-047
- **Found:** 2026-08-11 · **Report:** `docs/qa/reports/2026-08-11-frontend-performance.md`

## Summary

After the 12-entry older page became visible, the previously anchored `timeline-seed-006` row moved from 38 px below the viewport top to 842 px. The operator lost their reading position even though the page itself loaded gap-free.

## Reproduction

- **Charter:** CH-021 · **Tour:** Garbage Tour
- **Environment:** Chromium / desktop / Wi-Fi-fast / en-US; isolated local daemon; deterministic ACP fixture session `sess-fceaf43e8d958f9c`.

1. Open the fixture's bounded 200-entry tail and press Home.
2. Record `timeline-seed-006` at index 0 and 38 px below the transcript viewport top.
3. Activate Load older messages and wait for the 12-entry page to render.

**Expected:** The same stable ID remains at the same visual offset while its logical index changes to 12.
**Actual:** Its measured offset jumps from 38 px to 842 px.

## Evidence

- `docs/qa/evidence/2026-08-11-frontend-performance/long-transcript-older-page-anchor.png`
- `docs/qa/evidence/2026-08-11-frontend-performance/long-transcript-prepend-anchor-fixed.png`
- `docs/qa/evidence/2026-08-11-frontend-performance/long-transcript-virtualization.har`

## Fix

- **Root cause:** Prepending the older page changes the row count, removes the leading pagination control, and initially positions variable-height rows from estimates. The custom virtual scroll ownership had no durable representation of the user's measured browser anchor, so its internal estimate-based corrections could settle at a different visual position.
- **Correction:** Load older now publishes the stable message ID, measured offset, and current count to the thread XState Store. Once the immutable transcript grows, a layout-owned effect brings that ID into the virtual range and reconciles its measured offset across animation frames until variable-height measurement settles. DOM refs remain limited to imperative measurement; lifecycle state is owned by store events.
- **Fix commit:** `7d80c60`.
- **Regression test:** The canonical component suite covers older-page plus live-SSE anchoring, but jsdom could not reproduce the browser measurement drift. The browser invariant is tracked in `docs/qa/automation-backlog/session-timeline-virtual-state.md`.

## Verification

- **Retested:** 2026-08-11, same 212-entry route and real browser · **Report:** `docs/qa/reports/2026-08-11-frontend-performance.md`
- **Result:** `timeline-seed-006` moved from index 0 to 12 while its measured viewport offset remained 38.08 px. Eighteen rows were mounted during settlement, and the route emitted no page error or React lifecycle warning.
