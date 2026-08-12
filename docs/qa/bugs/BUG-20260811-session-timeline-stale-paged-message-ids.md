# BUG-20260811-session-timeline-stale-paged-message-ids: Older history loads but remains invisible

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Rafa
- **Journey Step:** J-14 page older transcript history
- **Scenarios:** RT-047
- **Found:** 2026-08-11 · **Report:** `docs/qa/reports/2026-08-11-frontend-performance.md`

## Summary

The older transcript request returned all 12 missing entries and the virtualizer grew from 200 to 212 items, but the rendered ID sequence still began at `timeline-seed-006`. The Load older control disappeared even though `timeline-seed-001` through `timeline-seed-005` were not reachable in the UI.

## Reproduction

- **Charter:** CH-021 · **Tour:** Garbage Tour
- **Environment:** Chromium / desktop / Wi-Fi-fast / en-US; isolated local daemon; deterministic ACP fixture session `sess-fceaf43e8d958f9c`.

1. Open the 212-entry fixture session; the REST tail returns 200 entries with `has_older=true` and `next_before_sequence=18`.
2. Press Home and activate Load older messages.
3. Observe the successful `GET .../transcript?before_sequence=18` response and inspect the first rendered message ID.

**Expected:** The 12 older entries prepend, `timeline-seed-001` is reachable, and the prior anchor shifts to its new index without a jump.
**Actual:** The virtual count grows and pagination completes, but rendered IDs remain the old 200-entry sequence beginning at `timeline-seed-006` index 0.

## Evidence

- `docs/qa/evidence/2026-08-11-frontend-performance/long-transcript-older-page-anchor.png`
- `docs/qa/evidence/2026-08-11-frontend-performance/long-transcript-older-page-fixed.png`
- `docs/qa/evidence/2026-08-11-frontend-performance/long-transcript-virtualization.har`

## Fix

- **Root cause:** `unstable_useThreadMessageIds` stores its ID sequence in an internal render-time `useRef`. In the compiled production route, the transcript and virtual count updated while that mutable ID source continued serving the previous tail sequence.
- **Correction:** The virtual rows now receive IDs directly from the immutable transcript projection already owned by TanStack Query and the session transcript context. No render state is held in a ref.
- **Fix commit:** `7d80c60`.
- **Regression test:** The canonical component suite already proves older-page plus concurrent-SSE anchoring, but jsdom did not reproduce the compiler/ref failure. Browser coverage is tracked in `docs/qa/automation-backlog/session-timeline-virtual-state.md`.

## Verification

- **Retested:** 2026-08-11, same route and real browser · **Report:** `docs/qa/reports/2026-08-11-frontend-performance.md`
- **Result:** The older request produced 212 reachable entries. Home rendered the initial assistant rows followed by `timeline-seed-001` at index 2; End rendered the final entry at index 211, with 11–13 mounted rows and no blank viewport.
