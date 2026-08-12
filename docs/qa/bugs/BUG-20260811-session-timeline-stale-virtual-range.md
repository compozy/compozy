# BUG-20260811-session-timeline-stale-virtual-range: Scrolling a long transcript can leave a blank viewport

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Rafa
- **Journey Step:** J-14 read a long transcript
- **Scenarios:** RT-047
- **Found:** 2026-08-11 · **Report:** `docs/qa/reports/2026-08-11-frontend-performance.md`

## Summary

A 200-entry session tail initially rendered at the live edge, but PageUp and Home could move the real scroller while the row subtree retained the previous virtual range. The viewport then showed empty space instead of the messages at the new scroll position.

## Reproduction

- **Charter:** CH-021 · **Tour:** Garbage Tour
- **Environment:** Chromium / desktop / Wi-Fi-fast / en-US; isolated local daemon; deterministic ACP fixture session `sess-fceaf43e8d958f9c`.

1. Open the fixture session with more than 200 durable transcript entries.
2. Confirm that the initial tail renders at the end with a bounded number of DOM rows.
3. Focus the transcript viewport and press PageUp repeatedly or Home.

**Expected:** The visible rows follow the new virtual range and the viewport remains readable.
**Actual:** The scroll position changes but the row subtree keeps the tail range, leaving the viewport blank.

## Evidence

- `docs/qa/evidence/2026-08-11-frontend-performance/long-transcript-tail-virtualized.png`
- `docs/qa/evidence/2026-08-11-frontend-performance/long-transcript-top-before-load.png`
- `docs/qa/evidence/2026-08-11-frontend-performance/long-transcript-older-page-fixed.png`
- `docs/qa/evidence/2026-08-11-frontend-performance/long-transcript-virtualization.har`

## Fix

- **Root cause:** `ThreadMessages` received the TanStack `Virtualizer` instance itself. That instance keeps a stable identity while mutating its visible range internally, so the React Compiler could reuse the child subtree even after the external-store notification rerendered the controller.
- **Correction:** The controller is an explicit compiler boundary and now publishes immutable `VirtualItem[]`, total size, and the measurement callback. The row component receives values whose identity changes with the visible range instead of reading a mutable library instance.
- **Fix commit:** `7d80c60`.
- **Regression test:** The jsdom candidate was rejected because it stayed green with the faulty mutable boundary. Canonical browser coverage is tracked in `docs/qa/automation-backlog/session-timeline-virtual-state.md`.

## Verification

- **Retested:** 2026-08-11, same 212-entry route and real browser · **Report:** `docs/qa/reports/2026-08-11-frontend-performance.md`
- **Result:** Home, PageDown, PageUp, and End published the correct row ranges without blank space. Only 11–13 message rows remained mounted.
