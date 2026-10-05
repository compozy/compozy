---
id: ET-web-route-chrome-topbar
area: ET
title: Unified window head absorbs PageHead
persona: Bruno
journey: J-marketplace-acquisition
expected: Every open desktop window owns one 48px unified head — identity (a 26px identity well + title for root windows, a window-local drill-in trail, or a document self-title with a state glyph), then status + ≤2 actions, then a hairline and the quiet Minimize · Zoom · Close icon controls (in the deck row instead when the window has ≥2 tabs) — with an optional 44px toolbar for peer views and listing tools; route identity renders once (no body PageHead / accent tile / workspace-prefixed breadcrumb); blurred windows dim identity and trail without a border change; focusing a window makes its head and URL authoritative without creating a second shell-level title.
entry_points: web desktop windows; any windowed catalog or detail route
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-web-catalog-navigation; ET-web-tasks-mode-url; ET-web-jobs-triggers-catalog
---

Added by Route Chrome alignment (2026-07-17). Flag only — retest in the next QA cycle.

Verify against `docs/design/opendesign/os/pagehead-redesign.html` (§02–§05), which owns the
unified window head contract.

QA impact 2026-07-20: OS Shell Task 08 absorbed PageHead into the window head — 44px identity,
optional 38px strip, window-local drill-in crumbs (no `compozy /` workspace prefix), document
session self-title. Reset to `untested` for the next QA cycle.

QA impact 2026-07-20: OS Shell Task 04 deleted the global `TopbarShell`. Route identity and
actions now live in each window's `TopbarSlotProvider`.

QA impact 2026-07-20: Peer RouteNav (Tasks modes · Marketplace kinds) moved from the 38px
tools strip into `TopbarSlotValue.nav` (after identity in the 44px head). Strip is tools-only.
Reset to `untested` for the next QA cycle.

qa-impact: 2026-09-30 shell rail v2 (flat topbar, left dock rail, gutterless tiling, browser-tab deck, light/dark theme). Head 44 → 48px with the identity well, traffic-light squares → quiet trailing icon controls, the 38px strip → the 44px window toolbar (views lead it; peer RouteNav no longer sits in the head). Verify against `docs/design/opendesign/design-system/os-shell.html` §04 and the shell-rail prototype.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
