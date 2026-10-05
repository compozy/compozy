---
id: ET-web-page-content-gutter
area: ET
title: Main-pane routes share one content gutter
persona: Bruno
journey: J-marketplace-acquisition
expected: Catalog, settings, home, and entity-detail routes in the main pane share the same horizontal inset (`px-9` + `max-w-content-max` via `PageContent` / `ListingPage` / `PageShell`). Left edges of PageHead titles align when navigating Agents → Home → Settings → a detail route. Session chat panes keep pane-local padding and are exempt.
entry_points: ListingPage catalogs; PageShell settings/home/mcp; entity detail shells
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-web-route-chrome-topbar; ET-web-jobs-triggers-catalog
---

Added by unified page gutters (2026-07-17). Flag only — retest in the next QA cycle.


2026-09-27 scope update: current coverage follows the surviving product surfaces; a fresh walk is required.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
