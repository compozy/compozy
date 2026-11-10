---
id: ET-site-marketplace-catalog
area: ET
title: Public /marketplace renders the checked-in catalog snapshot in the redesigned browse and detail pages
persona: Dora
journey: J-evaluate-compozy-beta
expected: /marketplace renders one searchable catalog from the checked-in v3 snapshot and matches the marketplace draft states browse, category, search and empty: featured entries, categories with counts, "View all" and flat search on the live catalog. /marketplace/<entry_id> and /marketplace/bundled/<name> match the detail state: identity, install command with copy, contents, provenance and related entries. Retired kind paths are not found. No old feed fallback, invented runtime state, popularity or secret values appear.
entry_points: compozy.com /marketplace; /marketplace/context7; /marketplace/herdr-bridge; /marketplace/bundled/spec-cycle; draft web/src/drafts/marketplace (states browse, category, search, empty, detail)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-09-13-marketplace-catalog/site-readback.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-site-docs-single-tree-ia
---

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

QA impact 2026-11-10 (site redesign, E2E-003): /marketplace, entry and bundled pages were rebuilt from the approved
marketplace draft (stories US-005 and US-006) on the same checked-in catalog snapshot. Walk each state at 1440×900 and
390×844 in the dark theme against the draft, with no console error and no horizontal overflow. The earlier skip is
history; this revised contract requires a fresh walk.
