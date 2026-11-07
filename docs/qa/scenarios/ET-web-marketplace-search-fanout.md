---
id: ET-web-marketplace-search-fanout
area: ET
title: Search the extension catalog across sources
persona: Bruno
journey: J-marketplace-acquisition
expected: Search filters the single catalog across its sources. Source diagnostics distinguish unavailable data from an empty match; clearing search restores the catalog. Pagination restarts after a content-revision change without mixing old and new pages.
entry_points: /marketplace?q=<query>; Marketplace search field
qa_status: pass
bug_ids: BUG-20260714-keyboard-focus-invisible
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: /tmp/mpc-shots/s8/web-search-flat-payments.png
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-api-marketplace-namespace; ET-web-marketplace-landing-browse
---

Marketplace catalog task 01 (2026-09-12): Search by name and description, clear a zero-result query, page through results, change the source revision before continuation, and verify the restarted list. Keep the complete installed shelf independent of filtered catalog rows.

Execution is deferred to tasks 09/10 by the loop delivery contract. Earlier evidence and notes below describe the previous surface and do not verify this contract.


Added by marketplace Task 06. Exercise VC04 and VC05 with a deterministic per-kind source failure and prove the browser issues one grouped request rather than four independent searches.

Historical QA note: retained sibling results, truthful stale/error isolation, and keyboard focus passed.

QA impact 2026-07-17: default listing view is now rows with optional cards; reset to confirm
partial-failure isolation still holds in both views.

QA impact 2026-07-18: web search fan-out ended with the grouped landing. The scenario is rescoped to
the active kind route and its selected Marketplace or Installed scope; daemon grouped discovery
remains covered by the API/CLI scenarios.

QA impact 2026-07-18: active-kind results now load server-owned continuation pages. Verify `Load
more`, retry after a continuation failure, exact versus loaded-only counts, and no duplicate cards.

QA impact 2026-07-18: remote skill continuation now uses a bounded one-row boundary check instead
of refetching the consumed prefix. Verify four or more pages remain ordered, a changed boundary is
rejected, Installed search stays local, and browser back does not get overwritten by stale debounce.

QA impact 2026-07-18: Market scope now preserves the server-owned result page without applying a
second literal substring filter. Verify a registry-ranked skill remains visible when the query is
matched by remote semantics rather than text copied into the listing projection.

QA impact 2026-07-18: pressing `/` outside editable controls focuses the active Marketplace kind
search. Verify the shortcut from the page body while preserving normal typing inside form controls.

QA impact 2026-08-02: the active-kind union lost the retired kind. Reset to verify isolation,
pagination, keyboard focus, and recovery across the surviving three kinds.

QA impact 2026-08-20: Marketplace kind ListingToolbar search height now uses `--height-search`
(28px) to match RouteNav and scope pills. Reset the kind-search chrome walk.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

## QA impact 2026-11-07 — marketplace plugin catalog (spec `marketplace-plugin-catalog`, S8)

Top search still replaces the category sections with one flat result list across the whole
catalog (US-003). Each result now shows its category word (`category_label`) instead of tier/author
for official entries; community entries keep their tier word. Search with a category word (for
example "design") and verify matches still come from name and description, not from the category
id, and that clearing search restores the sections. Capture the flat results.

### Walk result 2026-11-07 (S8 integrated lab, `feat/mpc-s8` with S1–S7 + S9)

Isolated lab (own home, ports 49760/49761, worktree web dev server), default sources plus a local `lab-plugins` folder source; cold refresh of the default sources took ~65.7 s (compozy-catalog 18, claude-plugins-official 316, compozy-plugins 333). Screenshots and CLI captures are lab-local in `/tmp/mpc-shots/s8/` (not committed).

PASS — `/marketplace?q=payments` replaces the sections with five flat results across sources, each
showing its category word (Finance & Payments, Cloud & Infrastructure, More); matches come from name
and description.
