---
id: ET-web-marketplace-search-fanout
area: ET
title: Search the extension catalog across sources
persona: Bruno
journey: J-marketplace-acquisition
expected: Search filters the single catalog across its sources. Source diagnostics distinguish unavailable data from an empty match; clearing search restores the catalog. Pagination restarts after a content-revision change without mixing old and new pages.
entry_points: /marketplace?q=<query>; Marketplace search field
qa_status: untested
bug_ids: BUG-20260714-keyboard-focus-invisible
fix_status: fixed
retest_status: untested
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/web/marketplace-skill-stale-served.png;/Users/pedronauck/Dev/compozy/compozy/.tmp/bug-20260714-focus/focused.png;/Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/extension-charters.json;/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-08-02-bundles-removal.md
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
