# BUG-20261005-loop-session-filter-disappears: The session-id filter disappears before the operator can type

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Marina
- **Journey Step:** J-03, find runs started by a particular session
- **Scenarios:** LP-008
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

CH-002, Interrupt Tour, 430x932 touch viewport at DPR 3, 4G, en-US.
From an unfiltered Runs list, open Add filter and choose Session id. The new text field
disappears and becomes an Origin=Session chip. The operator cannot enter the requested id.
Repeat after reloading the unfiltered page: the same result occurs.

## Evidence

`loops-filter-clear-reload.json` proves 35 restored rows and no origin filter after refresh.
`loops-filter-session-field.json` records the exact Session id option selected; the settled
receipt `loops-filter-session-field-settled.json` and screenshot
`loops-filter-session-id-missing.png` show only Origin=Session and no text field. Evidence
lives under `docs/qa/evidence/2026-10-02-untested/`. The four-frame
`loops-filter-trail-marina-fixed` recording is closed before repair.

This is distinct from failed clearing: the independent window read at revision 359 and
the reload prove that clearing persisted. Adding Session id deliberately selects session
origin, but loses the uncommitted text chip during that query transition.

## Fix

- **Root cause:** Runs removes its toolbar while a new server query loads. This destroys
  the local empty text-chip draft, which cannot be recovered from the committed URL.
- **Scope:** Preserve the existing toolbar and draft through query transitions, following
  the catalog's established lifetime. The existing shared filter search also stops forcing focus back on blur; public APIs are unchanged.
- **Regression test:** Existing `web/e2e/__tests__/loops.spec.ts`, real-daemon Runs filtering
  and continued session-id input across responses and reload.
- **Fix commit:** pending.
- **Retest:** pending.

## Second cause isolated

Keeping the toolbar mounted alone still fails the unchanged real-daemon E2E before a
textbox can be found. The filter component also treats its optimistic committed value
as if it had already arrived through route props. Its own local render therefore
replaces the empty draft with the old URL, followed by an origin-only chip when navigation
settles. Track the last observed props separately: an unchanged old prop is not an external
navigation, and an acknowledgement of the requested state preserves the chip identity.
The continued-typing and reload assertions remain unchanged.

## Focus ownership isolated

After preserving the draft, the unchanged E2E still failed initial focus. The focus
trace in loops-session-filter-focus-owner-settled.json shows repeated search-field
refocusing during the option click, then focus lost to BODY as the popup closes.
Guarding the blur handler with the open state was insufficient: the reentrant focus
events begin before close. Remove that forced blur refocus and its now-unused ref;
the existing input and menu focus behavior then succeeds without an added focus
handoff or context state.

The existing real-daemon suite passes initial focus, uninterrupted session-review
typing, successful filtered response, persisted value after reload, and Escape-to-trigger.
The adjacent catalog search canary also passes (loops-session-filter-e2e-minimal-focus.json).
Temporary diagnostic logging and the experimental handoff were removed. A cached
--only build did not include the UI dependency edit; the accepted build explicitly
rebuilt current sources (loops-session-filter-minimal-focus-build.json).
Marina's fresh 430x932 touch / 4G walk repeats focus, uninterrupted typing, reload,
Escape and Clear filter successfully on index-Ba1DLFXm.js. Independent UDS confirms
35 unfiltered rows and zero for the selected session. Evidence:
loops-session-filter-marina-verified.json/png and loops-session-filter-final-*-uds.json.
The nine-frame loops-filter-trail-marina-verified recording is closed.
The final delivery gate and fix commit remain pending.
