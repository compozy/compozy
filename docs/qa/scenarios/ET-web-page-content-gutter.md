---
id: ET-web-page-content-gutter
area: ET
title: Main-pane routes share one content gutter
persona: Bruno
journey: J-marketplace-acquisition
expected: Catalog, settings, home, and entity-detail routes in the main pane share the same horizontal inset (`px-9` + `max-w-content-max` via `PageContent` / `ListingPage` / `PageShell`). Left edges of PageHead titles align when navigating Agents → Home → Settings → a detail route. Session chat and network channel panes keep pane-local padding and are exempt.
entry_points: ListingPage catalogs; PageShell settings/home/sandbox/mcp; entity detail shells
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-route-chrome-topbar; ET-web-jobs-triggers-catalog
---

Added by unified page gutters (2026-07-17). Flag only — retest in the next QA cycle.
