---
id: ET-web-marketplace-kind-navigation
area: ET
title: Navigate Marketplace kinds from the default entry
persona: Bruno
journey: J-marketplace-acquisition
expected: Entering Marketplace through the sidebar or `/marketplace` lands on Skills in Installed scope; RouteNav switches among exactly Extensions, Skills, and MCPs, preserves explicit `tab=market`, no Bundle route exists, and detail Back returns to the same scope.
entry_points: Marketplace sidebar item; /marketplace; /marketplace/skills; /marketplace/extensions; /marketplace/mcps; retired /marketplace/bundles
qa_status: pass
bug_ids: BUG-20260802-retired-marketplace-kind-alias
fix_status: fixed
retest_status: pass
fix_commits: 7701a3f
evidence: /Users/pedronauck/dev/qa-labs/compozy-critical-runtime-ui-fixes-20260807-225222-371495-lab/qa-artifacts/qa/marketplace-extension-evidence.md
last_report: docs/qa/reports/2026-08-07-critical-runtime-ui-fixes.md
overlaps: ET-web-marketplace-landing-browse; ET-web-route-chrome-topbar
---

Added by the unified Marketplace hard cut. Walk redirect-mediated entry, every kind link, a detail
deep link, then a non-Marketplace sibling route to exercise retained route context.

QA impact 2026-08-02: corrected the stale four-kind expectation after the Bundle product cut.
