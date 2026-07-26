---
id: ET-marketplace-kill-switch
area: ET
title: Remove a pulled catalog entry from every surface
persona: Vera
journey: J-extension-policy-admin
expected: An entry removed from the curated feed disappears from search, browse, and detail on web, CLI, and API after refresh (and within TTL without one), while the already-installed capability remains fully manageable.
entry_points: agh marketplace refresh -o json; POST /api/marketplace/refresh; /marketplace; agh config set marketplace.catalog.base_url (isolated feed)
qa_status: untested
bug_ids: BUG-20260715-marketplace-config-set-live; BUG-20260715-config-set-late-metadata
fix_status: fixed
retest_status: pending fail-closed OAuth catalog schema validation
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/notes/marketplace-config-set-live.json; /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-kill-switch.json
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: ET-cli-marketplace-refresh; MS-marketplace-catalog-live-config
---

Added by marketplace Task 10 (QA planning). Covers Safety Invariant 3 / ADR-008: replace-on-refresh
prune is the curation kill-switch. Use an isolated local feed: install an entry, remove it from the
feed document, refresh, then prove absence on all discovery planes plus continued list/edit/remove/
authorize of the installed item (Safety Invariant 12). Also confirm a failed refresh never prunes —
the prior projection serves marked stale.

2026-07-15 final replay: a successful zero-entry skill refresh removed the catalog item from CLI,
HTTP, and Web while the installed skill remained enabled and manageable. After the item was restored,
a 503 refresh retained it with `stale=true` and the Web displayed an out-of-date alert. The fixture
was returned to normal and the final refresh cleared stale state.

QA impact 2026-07-16: serve an MCP OAuth entry missing a metadata source/direct endpoint pair and
entries with relative or non-HTTP endpoint URLs. Refresh must reject the document, preserve the last
valid projection, and expose stale/error truth rather than an uninstallable entry.
