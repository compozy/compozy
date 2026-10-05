# BUG-20261005-loop-search-loses-focus: Loop search loses focus while loading results

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Lea; keyboard users
- **Journey Step:** J-01, search the Loop catalog
- **Scenarios:** LP-001
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Editing a catalog query drops keyboard focus from the search field. The operator must
click it again to continue typing whenever a new server query loads.

## Reproduction

- **Charter:** CH-001 · **Tour:** Feature Tour
- **Environment:** laptop 1512×862, DPR 2, en-US, Wi-Fi; Studio Operations / resume-editorial

1. Focus Search loops.
2. Type or delete a character to request an uncached query.
3. Continue typing without clicking the field again.

**Expected:** Focus remains in Search loops while the catalog refreshes.
**Actual:** The active element becomes BODY, so subsequent typing misses the search field.

## Evidence

- docs/qa/evidence/2026-10-02-untested/loops-search-keyboard-diagnosis.json
- The separate driver Cmd+A selection failure is not used as product evidence. A native
  Backspace changed the input before focus moved from INPUT to BODY.

## Fix

- **Root cause:** LoopsCatalogLocation removes its entire toolbar whenever the catalog
  query isLoading, including on every uncached search, unmounting the focused input.
- **Scope:** Keep the toolbar mounted whenever a project is selected. The body retains
  its existing loading state. No focus restoration workaround or cache change.
- **Fix commit:** pending
- **Regression test:** Existing web/e2e/__tests__/loops.spec.ts owns focus continuity
  across actual daemon search responses and continued keyboard input.

## Verification

- **Retested:** pending

## First repair replay

Keeping the toolbar mounted preserves focus after the first response, but continued rapid
typing exposes a second cause in the same search binding: the input value comes directly
from asynchronous route state. Pending navigation replaces newly typed text with the older
query. The unchanged E2E types `loop` and receives `lop`; the real persona continuation
types `studio-intake-51` and receives partial `sio...` values while focus remains in INPUT.

The completed repair also uses the existing useDebouncedInput draft/commit owner, as the
Agents and Automation catalogs already do. Clearing all filters resets the draft and its
pending commit together. No test delay or manual refocus is added.

Evidence: loop-search-focus-e2e-green.json (failed; filename is intent, not verdict),
loops-catalog-fixed-lea-search-observation.json. The 17-frame recording is closed before repair.


## Completed replay — 2026-10-05

The unchanged real-daemon E2E passes (loop-search-focus-e2e-complete.json). In Lea's fresh
Chrome session, all sixteen per-character insertions accumulate exactly studio-intake-51
with focus remaining in INPUT; the server receives one complete query and returns the
off-page definition. Reload preserves its query and result, confirmed by independent HTTP.
The 35-frame loops-catalog-complete-lea recording is closed. Fix commit is pending.

Evidence: loops-catalog-complete-lea-search.json, search/search-reloaded PNGs and
loops-catalog-complete-search-readback.json under docs/qa/evidence/2026-10-02-untested/.
