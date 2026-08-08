---
id: ET-web-marketplace-landing-browse
area: ET
title: Enter and browse the Web marketplace
persona: Bruno
journey: J-marketplace-acquisition
expected: The Marketplace sidebar entry and `/marketplace` route land on Skills in Installed scope; choosing Marketplace records `tab=market`, and the shared kind template remains truthful and keyboard operable at desktop and mobile widths.
entry_points: /marketplace; /marketplace/skills; Marketplace sidebar item
qa_status: pass
bug_ids: BUG-20260714-keyboard-focus-invisible
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-critical-runtime-ui-fixes-20260807-225222-371495-lab/qa-artifacts/qa/marketplace-installed-default.png
last_report: docs/qa/reports/2026-08-07-critical-runtime-ui-fixes.md
overlaps: ET-api-marketplace-namespace; ET-web-marketplace-search-fanout
---

Added by marketplace Task 06. The next Web QA cycle should compare the landing against VC01, VC02, VC03, and VC06, including installed and update states with an active workspace.

Historical QA note: full-identity concurrent action state and extension Update routing remain pending.

QA impact 2026-07-16: pending actions now use `(kind, entry_id)` with independent overlap counts,
and installed extension Update uses the lifecycle PUT instead of the install POST; reset for the next
browser QA cycle.

QA impact 2026-07-16: a fresh default catalog now exposes Context7, Repository Orientation, and
Documentation Writer without a search query; verify their cards and exact detail routes before any
install state exists.

QA impact 2026-07-17: Marketplace landing and kind browse now ship Rows/Cards ViewToggle with URL
`view` persistence (default rows) and ListingRow parity; verify both views and install/manage
actions in each.

QA impact 2026-07-17: Marketplace kind navigation is now RouteNav links (not PillGroup buttons) under
the route-chrome topbar; identity/count live in PageHead. Flag only — next QA cycle.

QA impact 2026-07-18: the grouped landing was removed. This scenario now owns entry through the
sidebar or index redirect and the default Skills kind browse surface; cross-kind navigation is
tracked by ET-web-marketplace-kind-navigation.

QA impact 2026-08-02: the shared Marketplace shell now exposes exactly three kinds. Reset for the
adjacent acquisition canary.
