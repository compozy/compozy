---
id: ET-web-marketplace-kind-navigation
area: ET
title: Navigate Marketplace kinds from the default entry
persona: Bruno
journey: J-marketplace-acquisition
expected: Entering Marketplace through the sidebar or `/marketplace` lands on Skills, RouteNav switches among exactly Extensions, Skills, and MCP, no Bundle route exists, and the breadcrumb clears after leaving the catalog.
entry_points: Marketplace sidebar item; /marketplace; /marketplace/skills; /marketplace/extensions; /marketplace/mcp; retired /marketplace/bundles
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-web-marketplace-landing-browse; ET-web-route-chrome-topbar
---

Added by the unified Marketplace hard cut. Walk redirect-mediated entry, every kind link, a detail
deep link, then a non-Marketplace sibling route to exercise retained route context.

QA impact 2026-08-02: corrected the stale four-kind expectation after the Bundle product cut.
