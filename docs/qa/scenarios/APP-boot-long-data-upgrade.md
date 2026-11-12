---
id: APP-boot-long-data-upgrade
area: APP
title: A long data upgrade shows real progress instead of a frozen window
persona: Bruno
journey: J-desktop-update-moment
expected: Upgrading the app over a home with thousands of retained sessions at an older schema or transcript projection keeps the boot window visibly alive. It reads "Updating CompozyOS" with the from → to version, marks Upgrade data as the current step, counts "N of M sessions" with a percentage and an estimate, explains that the upgrade runs once, and surfaces logs and diagnostics if progress stops. It ends in the product window with every session intact.
entry_points: Desktop app update or first launch of a newer app build over an existing home; desktop-bootstrap.jsonl; ~/.compozy/restarts/<operation>.json
qa_status: untested
bug_ids:
fix_status: 
retest_status: 
fix_commits: 
evidence:
last_report:
overlaps: APP-runtime-update-app-owned; APP-update-recovery-state
---

Origin: #1549. Upgrading a real workstation from v0.5.x to v0.6.0 left the window on "Preparing
CompozyOS · Installing runtime" for about 30 minutes. The replacement daemon was upgrading the
transcript projection of about 2,550 session databases (`upgradeBootSessionDatabases`), and none of
that progress reached the screen.

Lab setup: an isolated home (`eng-qa-bootstrap`) seeded with at least 1,500 sessions written by the
previous release, so each session database is at the older schema and projection version. Copying a
real long-running home also works. The lab needs the desktop app built from the previous release
and the app built from this change, both pointing at the lab home.

Walk:

1. Launch the new app over the seeded home. The boot window opens at 880 × 540. It reads
   "Updating CompozyOS" with the `<old> → <new>` pill, and the steps Verify, Install, Upgrade data,
   Start and Connect.
2. While the replacement upgrades sessions, `desktop-bootstrap.jsonl` records `provision` events
   with `stage: "migrate"`, `migration: "session_history_upgrade"`, a fixed `total` equal to the
   retained session count, and a rising `completed`. The window shows the same counts with "Upgrading
   sessions", fills the point field in step, and shows an estimate after about 10 seconds.
3. After 30 seconds the panel explains "You have a lot of history" with Open logs. Open logs opens
   the logs folder.
4. "What's new" opens the target release's GitHub notes in the browser and nothing else.
5. The waiting game runs by itself in the left column. Space takes control, a miss shows the Failed
   face and the next agent takes over, and reduced motion leaves it still.
6. Stall check (separate run): suspend the replacement daemon during the upgrade. After two
   minutes the window turns amber with "No progress for 2 minutes", Open logs and Diagnostics.
   The status line reads "Waiting" beside the total boot clock. Diagnostics loads the report in the
   panel without an error, and Back to progress returns to the counts and the notice. Resume the
   daemon and the notice clears once counts advance. Left suspended past the CLI's 5-minute window, the boot ends in the
   error layout with the Upgrade data step failed.
7. The upgrade ends in the product window. `compozy app status -o json` validates against the
   schema, and every seeded session opens with its transcript.

Owning suites: `TestBootSessionRepair` (per-session progress, cleared on exit),
`TestGlobalDBUsageCostRepair` (usage cost repair progress), `TestDesktopRuntimeBootstrapReplacement`
(counts, stage name and version transition on bootstrap events), desktop `bootstrap-state`,
`bootstrap-event`, `boot-journey`, `boot-runner-engine` and `boot-control`.
