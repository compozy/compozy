---
id: ET-web-route-chrome-topbar
area: ET
title: Unified window head absorbs PageHead
persona: Bruno
journey: J-marketplace-acquisition
expected: Every open desktop window owns one 44px unified head (traffic lights · quiet glyph + title or window-local drill-in trail · peer RouteNav tabs immediately after identity when the route has siblings · status + ≤2 actions) with an optional 38px context strip for listing tools only (search/filter/sort/scope — never peer route tabs); route identity renders once (no body PageHead / accent tile / workspace-prefixed breadcrumb); document/session windows self-title with a state mark; focusing a window makes its head and URL authoritative without creating a second shell-level title.
entry_points: web desktop windows; any windowed catalog or detail route
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-catalog-navigation; ET-web-tasks-mode-url; ET-web-jobs-triggers-catalog
---

Added by Route Chrome alignment (2026-07-17). Flag only — retest in the next QA cycle.

Verify against `docs/design/opendesign/os/pagehead-redesign.html` (§02–§05), which owns the
unified window head contract.

QA impact 2026-07-20: OS Shell Task 08 absorbed PageHead into the window head — 44px identity,
optional 38px strip, window-local drill-in crumbs (no `agh /` workspace prefix), document
session self-title. Reset to `untested` for the next QA cycle.

QA impact 2026-07-20: OS Shell Task 04 deleted the global `TopbarShell`. Route identity and
actions now live in each window's `TopbarSlotProvider`.

QA impact 2026-07-20: Peer RouteNav (Tasks modes · Marketplace kinds) moved from the 38px
tools strip into `TopbarSlotValue.nav` (after identity in the 44px head). Strip is tools-only.
Reset to `untested` for the next QA cycle.
