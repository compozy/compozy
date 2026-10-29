---
id: ET-web-catalog-navigation
area: ET
title: Navigate the desktop app registry
persona: Bruno
journey: J-marketplace-acquisition
expected: The dock, Go menu, command palette, tooltips, and window titles use the canonical desktop app registry and open or focus one window per app. Agents, Tasks, Loops, Automations, Marketplace, Vault, Terminal, Sessions, and Home remain reachable. Settings opens from the dock foot (and the CompozyOS mark menu). Child routes preserve the owning app window and browser history. Removed product apps have no dock item, palette hit, app descriptor, or live route.
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

The dock, Go menu, command palette, tooltips, and window titles use the canonical desktop app registry and open or focus one window per app. Agents, Tasks, Loops, Automations, Marketplace, Vault, Terminal, Sessions, and Home remain reachable. Settings opens from the menubar cog. Child routes preserve the owning app window and browser history. Removed product apps have no dock item, palette hit, app descriptor, or live route.

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

qa-impact: 2026-09-30 shell rail v2. Settings left the menubar cog for the dock foot. Already untested; expectation updated.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

QA impact 2026-10-08 (Automations spec task 07): the Jobs and Triggers web apps merged into one Automations window. Web entry points moved to `/automations` (old `/jobs*` and `/triggers*` links redirect until v0.5.0); walk through the new route. Daemon, CLI and API behavior in this scenario is unchanged.

QA impact 2026-10-07 (memory removal): the Knowledge app and `/knowledge` were removed from the registry, so the dock, Go menu, and palette list one fewer launcher; `/knowledge` renders the standard not-found route and no Knowledge palette hit or descriptor remains. Stale skipped verdict reset to untested; the 2026-10-05 skip was specific to that cycle; no QA session ran.


QA impact 2026-10-17 (web performance): The empty desktop/composer and worktree dialog boundaries now load on demand. A cold app deep link must open the intended window without briefly mounting the empty composer; returning to an empty desktop must still show the launcher. Re-walk worktree dialogs on first open and existing reload/window lifecycle. Initial Tasks exploration used the first split build, so the final-source result remains assigned to CI.

Scope, measured results, and verification limits: [Web performance report](../reports/2026-10-17-web-performance.md). Historical scenario verdicts above are not new verification of this revision. Heavy local gates are deferred to CI by explicit user instruction.

Recovery follow-up: block the deferred launcher/dialog JavaScript request and confirm a contained recovery surface. Closing the unavailable dialog keeps the desktop reachable; Reload fetches the feature again and preserves the ordinary window/launcher journey. The existing OS-shell/worktree E2E journeys own these network-failure legs in CI.


Issue #1034 targeted request-lifecycle walk (2026-10-29): capture the worktree
catalog WebSocket while opening, minimizing, restoring, and closing ordinary
windows. These visibility changes must not close or recreate that subscription.
Move only the isolated project's root away, reload, and verify that known missing
roots do not trigger worktree reads. A 410 discovered by an in-flight read must
stop retries and stream-triggered refetches. Restore the same folder and verify
that a fresh workspace catalog observation re-enables its worktree listing.
Evidence and bounded verification: [gone-resource report](../reports/2026-10-29-gone-resource-requests.md).

Targeted re-walk 2026-10-29: PASS for the #1034 request-lifecycle steps above.
The broader historical scenario is not re-certified by this bounded walk.
