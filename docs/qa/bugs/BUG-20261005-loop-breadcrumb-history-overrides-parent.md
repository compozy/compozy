# BUG-20261005-loop-breadcrumb-history-overrides-parent: An old navigation entry overrides the selected breadcrumb

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Marina
- **Journey Step:** J-03, return from Runs to the Loop catalog
- **Scenarios:** LP-web-runs-breadcrumb
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

Open Runs from the catalog, then a run. Deep-link back to `/loop-runs` without consuming the
window's navigation history. Tap Back one level to return to the catalog. The URL briefly
changes to `/loops` or immediately returns to `/loop-runs`, and the Runs page stays open.
The same mismatch can affect a selected parent crumb.

## Evidence

`loops-trail-parent-mismatch-setup.json` records the real navigation. Before the Back action,
`loops-trail-parent-before.json` shows both the current route and the last history entry as
`/loop-runs`. The browser receipt `loops-trail-parent-mismatch.json` records a successful
`window.navigate` with `mode=pop`, and `loops-trail-parent-after.json` confirms the daemon
consumed that entry while leaving the window on Runs. Receipts are under
`docs/qa/evidence/2026-10-02-untested/`; only one lab browser tab was connected.

## Root cause and repair

The routing coordinator treats every breadcrumb as a pop whenever history is nonempty,
although the app's selected parent can differ from that history entry. The daemon correctly
performs the requested pop. Compare the intended parent with the stack top before choosing
pop; otherwise navigate to the selected parent through the existing controller.

The existing `routing-coordinator.test.ts` suite owns URL/window reconciliation, including
matching and mismatched history. Fix commit: pending delivery gate.

Fresh Marina touch replay on 430x932 / 4G confirms Runs Back reaches the catalog despite
old history, Compare Back and its explicit run parent reach the same run, hidden Runs
returns to all 35 rows, and the Loops crumb opens the catalog. Direct links to Compare
and Runs preserve these destinations; the final catalog survives a fresh document.
Independent window-manager revision 398 retains /loops. Evidence:
loops-trail-marina-verified-detail.json, loops-trail-marina-verified-parent-chain.json,
loops-trail-marina-verified-deep-links.json and loops-trail-final-window-uds.json.
The exact nine-frame loops-filter-trail-marina-verified recording is closed.

