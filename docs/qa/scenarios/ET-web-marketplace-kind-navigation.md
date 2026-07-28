---
id: ET-web-marketplace-kind-navigation
area: ET
title: Navigate Marketplace kinds from the default entry
persona: Bruno
journey: J-marketplace-acquisition
expected: Entering Marketplace through the sidebar or `/marketplace` lands on Skills, RouteNav switches among all four kind routes, and the breadcrumb is exactly Home, linked Marketplace, and the active kind with no retained Marketplace crumb after leaving the catalog.
entry_points: Marketplace sidebar item; /marketplace; /marketplace/skills
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-marketplace-landing-browse; ET-web-route-chrome-topbar
---

Added by the unified Marketplace hard cut. Walk redirect-mediated entry, every kind link, a detail
deep link, then a non-Marketplace sibling route to exercise retained route context.
