---
id: ET-web-marketplace-landing-browse
area: ET
title: Enter and browse the Web marketplace
persona: Bruno
journey: J-marketplace-acquisition
expected: The sidebar and /marketplace open one catalog grouped by category: installed shelf, Featured when a curated source features entries, then each non-empty category in daemon order with up to six rows and View all · N (the daemon's full count). No source sections. View all opens /marketplace/categories/<id> with the full paged list, a category-scoped search and Load more; an unknown id shows "This category doesn't exist" with Back to Marketplace. Loading, stale, empty and narrow-window states remain truthful and keyboard operable.
entry_points: /marketplace; /marketplace/categories/<id>; Marketplace sidebar item; View all link
qa_status: fail
bug_ids: [BUG-20260714-keyboard-focus-invisible, BUG-20260913-marketplace-search-loading-visuals, BUG-20260913-marketplace-background-refresh, BUG-20261107-marketplace-empty-version-500, BUG-20261107-catalog-entries-uncategorized-duplicated]
fix_status: open
retest_status: pending
fix_commits: 8eeb8a38
evidence: /tmp/mpc-shots/s8 (web-browse-home-*-DEFECT-500.png, web-category-page-finance.png, web-category-not-found.png, web-add-ready-page.png)
last_report: docs/qa/reports/2026-09-13-marketplace-catalog.md
overlaps: ET-api-marketplace-namespace; ET-web-marketplace-search-fanout
---

QA 2026-09-13: the current hard-cut contract passed the scoped live/API/browser walks and applicable unchanged owning integration checks. See the dated report for exact evidence and boundaries; historical notes below do not redefine the current catalog.

Issue #644 regression walk: execution results and captured viewports are tracked in
[PR #648](https://github.com/compozy/compozy/pull/648). The canonical Marketplace `E2E-006` opens the
production daemon-served bundle with a healthy feed and a second source. Corrupt that source,
click Refresh, and verify only its name appears in the warning while both cached and healthy
entries remain. Repair it, click Retry, and verify the warning clears; repeat with Refresh.
Verify shipped SVG/PNG images finish decoding under the real production CSP, unsupported remote
icons make no request, and no CSP errors occur. Capture degraded and recovered viewport states.
The separate public GitHub acquisition CI probe verifies the official document and pinned archive
without credentials. The recovery/CSP journey first passed in
[CI run 34924580842](https://github.com/compozy/compozy/actions/runs/34924580842/job/104239985378)
(as E2E-005, before its identifier was made unique). That run did not retain the correct screenshot
artifact; PR #648 owns the corrected capture evidence and final-head checks. Earlier scenario
evidence does not verify this regression walk.


Marketplace catalog task 01 (2026-09-12): Browse the default catalog, narrow search, refresh a cached unavailable source, open the shelf, and check the one-column layout. Verify retired kind links show not-found with Back to Marketplace. Reopen a saved retired location and verify its location, geometry, desktop and unrelated windows remain intact until deliberate navigation.

Execution is deferred to tasks 09/10 by the loop delivery contract. Earlier evidence and notes below describe the previous surface and do not verify this contract.


Added by marketplace Task 06. The next Web QA cycle should compare the landing against VC01, VC02, VC03, and VC06, including installed and update states with an active workspace.

Historical QA note: full-identity concurrent action state and extension Update routing remain pending.

QA impact 2026-07-16: pending actions now use `(kind, entry_id)` with independent overlap counts,
and installed extension Update uses the lifecycle PUT instead of the install POST; reset for the next
browser QA cycle.

QA impact 2026-07-16: a fresh default catalog now exposes Context7, Repository Orientation, and
Documentation Writer without a search query; verify their cards and exact detail routes before any
install state exists.

QA impact 2026-07-17: Marketplace landing and kind browse now ship Rows/Cards ViewToggle with URL
`view` persistence (default rows) and ListingRow parity; verify both views and install/manage
actions in each.

QA impact 2026-07-17: Marketplace kind navigation is now RouteNav links (not PillGroup buttons) under
the route-chrome topbar; identity/count live in PageHead. Flag only — next QA cycle.

QA impact 2026-07-18: the grouped landing was removed. This scenario now owns entry through the
sidebar or index redirect and the default Skills kind browse surface; cross-kind navigation is
tracked by ET-web-marketplace-kind-navigation.

QA impact 2026-08-02: the shared Marketplace shell now exposes exactly three kinds. Reset for the
adjacent acquisition canary.


## QA impact and focused evidence 2026-11-02 — responsiveness audit

Browse and detail availability checks avoid repeatedly hashing unchanged cached packages while preserving current file/digest/error checks. The Web source was not changed. A package disappearing after a warm browse must become `package_unavailable`, and restoring approved bytes must recover availability without a remote fetch.

Focused verification passed under `-race`: `TestCatalogServiceDetailAndStatus` (`internal/marketplace/service_test.go`) completed in 5.964 s (`.cache/performance/extension-marketplace/marketplace-race-final.txt`). Its real SQLite/file fixture covers browse/detail agreement and recovery. `TestPackageCache` separately owns corruption, expected-digest and replacement behavior. The 8 MiB availability benchmark improved from 4,724,509 to 180,411 ns/op (five 200 ms samples, median); this is a backend availability measurement, not browser interaction latency.

Existing root CI mappings, not rerun by this worker: `TestCatalogServiceHTTPProjectionIntegration` (`internal/marketplace/service_integration_test.go`); browser `E2E-001 / E2E-005: operator browses independent sources and retains installed packages across source recovery` and `E2E-006: Retry recovers one failed source and shipped logos load under production CSP` (`web/e2e/__tests__/marketplace.spec.ts`). These remain the owners for source degradation/recovery and production-bundle rendering.

Historical browser verdicts remain unchanged. No current screenshot, keyboard, narrow-window, CSP or live source-recovery pass is claimed. Evidence and limitations: [extension/marketplace audit](../reports/2026-11-01-performance/extension-marketplace.md).

## QA impact 2026-11-07 — marketplace plugin catalog (spec `marketplace-plugin-catalog`, S8)

Browse is rebuilt around categories (US-001, US-002, US-004; `_uiux.md` S1, S2, S7). The
earlier "one source has no section header" contract is retired; source sections are deleted.

Walk with the default sources enabled in an isolated lab:

1. Open `/marketplace`. Verify order: installed shelf → Featured (only when non-empty) → categories
   in the fixed order, empty ones omitted. Each section is a labelled region named by its category,
   shows at most six rows in two columns (one column below `sm`), and **View all · N** matches
   `GET /api/marketplace/sections` `total` for that category. No row repeats an app that a curated
   entry supersedes.
2. Activate **View all** on a populated category (accessible name "View all {Category}"). The
   window location becomes `/marketplace/categories/<id>`, the title is the daemon label, and the
   count matches `compozy marketplace categories -o json`. Scroll to **Load more** and verify the next
   page appends without duplicates.
3. Type in "Search {Category}": results stay inside the category; an unmatched query reads
   `No results for "{q}" in {Category}`; clearing restores the full list.
4. Open `/marketplace/categories/apps`: "This category doesn't exist" with **Back to Marketplace**,
   which returns to Browse.
5. Capture Browse in light and dark, a category page and the not-found state.

### Walk result 2026-11-07 (S8 integrated lab, `feat/mpc-s8` with S1–S7 + S9)

Isolated lab (own home, ports 49760/49761, worktree web dev server), default sources plus a local `lab-plugins` folder source; cold refresh of the default sources took ~65.7 s (compozy-catalog 18, claude-plugins-official 316, compozy-plugins 333). Screenshots and CLI captures are lab-local in `/tmp/mpc-shots/s8/` (not committed).

- **FAIL — Browse home with the real catalog.** After the refresh, `GET /api/marketplace/sections` and
  the unfiltered list return 500 and Browse shows "The marketplace is unreachable"
  (BUG-20261107-marketplace-empty-version-500). Category pages for productivity, design, development,
  data and more fail the same way.
- PASS before the GitHub sources loaded (compozy-catalog + lab-plugins): sections in daemon order with
  View all · N, rows ≤ 6, Featured absent when empty (`web-add-ready-page.png`).
- PASS — category page `finance` (30, scoped search, two columns) and `more` (18) with title and count
  from the daemon; View all link named "View all More".
- PASS — `/marketplace/categories/apps` shows "This category doesn't exist" and Back to Marketplace
  returns to `/marketplace`.
- Degraded — all 18 compozy-catalog entries resolve to More and duplicate compozy-plugins apps
  (BUG-20261107-catalog-entries-uncategorized-duplicated).
- Not captured: Browse home light/dark with the full catalog (blocked by the 500).
