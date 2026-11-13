---
id: ET-window-layout-upgrade-v06
area: ET
title: Upgrade a v0.6 layout to tabs with a notice and counts
persona: Théo
journey: J-organize-tabbed-work
expected: On the first start of this build over a workspace whose stored layout is version 5, every former window opens as a tab or inside a split tab and no window, route or desktop is lost; the Web shell shows "Your windows are now tabs" with the number of floating and minimized windows that became tabs; Dismiss in one client hides the notice in every connected client; `compozy layout migration status -o json` reports `state: "migrated"`, `from_version: 5`, a `summary` whose counts match the seed, and `acknowledged` flips to true after the dismiss or `compozy layout migration ack`; `layout get` drops `migration` once acknowledged.
entry_points: first daemon start after upgrade; upgrade notice Dismiss; compozy layout migration status; compozy layout migration ack; compozy layout get; compozy__layout_get; GET/POST /api/workspaces/{id}/window-manager/migration
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-window-tab-v3-discard; ET-window-manager-multi-client; ET-window-manager-layout-recovery; ET-window-layout-upgrade-failure-retry
---

story: An operator upgrading from v0.6 opens CompozyOS and finds every window they had, now as tabs, with a short note explaining what changed.

scope: Snapshot v5 → v6 conversion at boot, the upgrade notice and its counts, cross-client acknowledgement, and the migration status surfaces. The failure path is ET-window-layout-upgrade-failure-retry.

Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build (unique home, port and socket). Either start a v0.6 binary on the lab home and build the layout by hand (two tiled windows split, one floating window, one minimized window, two windows snapped left/right, one pinned window), or seed a version 5 layout with `go run ./web/e2e/fixtures/retired-knowledge-window-seeder --home <home> --workspace <id> --floating <a> --minimized <b> --snapped <c>,<d> --pinned <e>` with the daemon stopped. Record the window ids and routes. Start this build. In two browsers on the workspace confirm: the tiled split is one tab with both windows; the floating and minimized windows are tabs after the tiled ones; the snapped pair is one split tab; the pinned window is a pinned tab unless it ended up inside a split; every route is unchanged. The notice reads "Your windows are now tabs" with the floating and minimized counts. `compozy layout migration status --workspace <id> -o json` shows `migrated`, `from_version: 5` and the summary counts. Dismiss in browser A: the notice disappears in browser B without a reload. Status now shows `acknowledged: true` and `compozy layout get -o json` has no `migration`. Restart the daemon: no second conversion, no notice.
