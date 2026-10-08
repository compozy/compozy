---
id: ET-web-catalog-navigation
area: ET
title: Navigate the desktop app registry
persona: Bruno
journey: J-marketplace-acquisition
expected: The dock, Go menu, command palette, tooltips, and window titles use the canonical desktop app registry and open or focus one window per app. Agents, Tasks, Loops, Jobs, Triggers, Marketplace, Vault, Terminal, Sessions, and Home remain reachable. Settings opens from the dock foot (and the CompozyOS mark menu). Child routes preserve the owning app window and browser history. Removed product apps have no dock item, palette hit, app descriptor, or live route.
entry_points: web desktop dock; command palette; dock-foot Settings; CompozyOS mark menu; Catalog and System destinations
qa_status: untested
bug_ids: BUG-20260802-retired-marketplace-kind-alias
fix_status: fixed
retest_status:
fix_commits: 7701a3f
evidence: /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-under-minute.json;/Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa;/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa;docs/qa/reports/2026-08-20-ui-normies-retry.md
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-web-marketplace-landing-browse; ET-web-extensions-manage
---

The dock, Go menu, command palette, tooltips, and window titles use the canonical desktop app registry and open or focus one window per app. Agents, Tasks, Loops, Jobs, Triggers, Marketplace, Vault, Terminal, Sessions, and Home remain reachable. Settings opens from the menubar cog. Child routes preserve the owning app window and browser history. Removed product apps have no dock item, palette hit, app descriptor, or live route.

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

qa-impact: 2026-09-30 shell rail v2. Settings left the menubar cog for the dock foot. Already untested; expectation updated.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

QA impact 2026-10-07 (memory removal): the Knowledge app and `/knowledge` were removed from the registry, so the dock, Go menu, and palette list one fewer launcher; `/knowledge` renders the standard not-found route and no Knowledge palette hit or descriptor remains. Stale skipped verdict reset to untested; the 2026-10-05 skip was specific to that cycle; no QA session ran.
