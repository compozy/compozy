---
id: ET-site-marketplace-catalog
area: ET
title: Public /marketplace renders the checked-in catalog snapshot in the redesigned browse and detail pages
persona: Dora
journey: J-evaluate-compozy-beta
expected: /marketplace renders one searchable catalog from the checked-in v3 snapshot and matches the marketplace draft states browse, category, search and empty: featured entries, categories with counts, "View all" and flat search on the live catalog. /marketplace/<entry_id> and /marketplace/bundled/<name> match the detail state: identity, install command with copy, contents, provenance and related entries. Retired kind paths are not found. No old feed fallback, invented runtime state, popularity or secret values appear.
entry_points: compozy.com /marketplace; /marketplace/context7; /marketplace/herdr-bridge; /marketplace/bundled/spec-cycle; draft web/src/drafts/marketplace (states browse, category, search, empty, detail)
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/site-redesign/qa/R4/ET-site-marketplace-catalog/walk.jsonl; .compozy/tasks/site-redesign/qa/R4/ET-site-marketplace-catalog/browse-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-marketplace-catalog/browse-1440-draft.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-marketplace-catalog/category-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-marketplace-catalog/search-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-marketplace-catalog/empty-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-marketplace-catalog/detail-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-marketplace-catalog/bundled-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-marketplace-catalog/extension-context7-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-marketplace-catalog/rewalk-hardload-390-site.jpg
last_report: docs/qa/reports/2026-11-10-site-redesign-r4.md
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

QA impact 2026-11-10 (review fixes R1, R4): picking a category from Featured ("View all") or the rail, "All" with or without a search, typing in the hero search, and a hard load of `/marketplace/?category=<id>#catalog` each land the catalog at the draft's position under the header.

QA result 2026-11-10 (R4 walk, `next start`, 1440 and 390, dark): pass. Browse, category, search ("postgres", 9 results), empty ("fax machine") and detail match their drafts (height within 24px); 357 entries in 12 categories plus More; `/marketplace/context7`, `/marketplace/herdr-bridge` and `/marketplace/bundled/spec-cycle` show identity, install or status command, contents, provenance and related entries; retired kind paths (`/marketplace/extension/…`, `/marketplace/skills/…`, `/marketplace/extensions/`, `/marketplace/plugin/…`) answer 404. Defect fixed during the walk: catalog jumps landed 88px lower than the draft because the page's scroll padding added to the draft's own offsets; after the fix every jump equals the draft's measured position at both widths.
