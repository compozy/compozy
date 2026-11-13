---
id: ET-window-layout-upgrade-v06
area: ET
title: Upgrade a v0.6 layout to tabs with a bell item and counts
persona: Théo
journey: J-organize-tabbed-work
expected: On the first start of this build over a workspace whose stored layout is version 5, every former window opens as a tab or inside a split tab and no window, route or desktop is lost; the Web shell's bell holds "Your windows are now tabs" with the number of floating and minimized windows that became tabs, a toast shows it once per client, and no inline notice appears; marking the item read in one client clears it from every connected client's bell; `compozy layout migration status -o json` reports `state: "migrated"`, `from_version: 5`, a `summary` whose counts match the seed, and `acknowledged` flips to true after the item is read or `compozy layout migration ack`; `layout get` drops `migration` once acknowledged.
entry_points: first daemon start after upgrade; the bell's layout-upgrade item; compozy layout migration status; compozy layout migration ack; compozy layout get; compozy__layout_get; GET/POST /api/workspaces/{id}/window-manager/migration
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

scope: Snapshot v5 → v6 conversion at boot, the bell item and toast with its counts, cross-client acknowledgement, and the migration status surfaces. The failure path is ET-window-layout-upgrade-failure-retry.

Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build (unique home, port and socket).

Seed, path A (preferred when a v0.6 binary is at hand): on a v0.6 home, desktop 1 holds two windows tiled side by side (a | b), one floating window (e), one minimized window (f) and one pinned window (g); desktop 2 holds exactly two windows snapped to the left and right halves (c, d) and nothing else tiled. Stop v0.6 and start this build on the same home.

Seed, path B (seeder): with this build, open the windows and arrange them so the seeder can rewrite them. The seeder turns every tab of the current layout into one full-area group of a version 5 layout (a split tab stays one split group), then moves the named windows. A snapped pair only converts into one split tab when the two windows are the only tiled content of their desktop, because full-area groups overlap and the converter keeps overlapping groups as separate tabs. So: desktop 1 has a tab with a | b split, and single-window tabs e, f, g (pin g); desktop 2 has single-window tabs c and d only. Stop the daemon and run `go run ./web/e2e/fixtures/retired-knowledge-window-seeder --home <home> --workspace <id> --floating <e> --minimized <f> --snapped <c>,<d> --pinned <g>`; it prints the stored value. Start the daemon.

Then, in two browsers on the workspace: desktop 1 shows g pinned at the left of the bar, the a | b split tab, and e and f as tabs after the tiled ones; desktop 2 shows one tab with c | d split. Every route is the one recorded before the upgrade. No inline notice shows; a toast and the bell item read "Your windows are now tabs" with "1 floating window and 1 minimized window are tabs now." Reload browser A: the toast does not show again and the item stays in the bell. `compozy layout migration status --workspace <id> -o json` shows `state: "migrated"`, `from_version: 5`, `acknowledged: false`, and a summary with `floating_to_tabs: 1`, `minimized_to_tabs: 1`, `islands_merged: 1`. Click the item in browser A's bell: it leaves A's bell, which stays open, and leaves browser B's bell on its next read without a reload, status shows `acknowledged: true`, and `compozy layout get --workspace <id> -o json` no longer has `migration`. Restart the daemon: no second conversion and no bell item. Control: repeat path B with a third tab left on desktop 2 next to c and d; c and d then arrive as separate tabs and `islands_merged` is 0.
