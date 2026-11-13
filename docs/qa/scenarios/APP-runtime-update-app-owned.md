---
id: APP-runtime-update-app-owned
area: APP
title: Update an app-provisioned runtime through one product action
persona: Bruno
journey: J-desktop-update-moment
expected: On an app-provisioned home, one Update activation installs the app package and its bundled runtime. The app remains open until native staging succeeds, restarts once, replaces only its proven owned runtime, and reports both observed versions. Explicit runtime-only updates retain the existing runtime path.
entry_points: Settings → General → Updates in Chrome or the app; compozy update; settings update HTTP and UDS routes; signed compatibility catalog and immutable release assets
qa_status: untested
bug_ids:
fix_status: 
retest_status: 
fix_commits: 
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-08-17-electron-shell.md
overlaps: APP-runtime-update-managed
---

PRD stories: US-016 (AC-1 one experience, AC-2 timing consent under in-flight work, AC-3
reconnect + new version; EC-1 fail → previous usable + diagnostics; EC-2 repeated decline, no
nagging; EC-3 ownership flip → managed behavior). BR-8/BR-9. Test ID: E2E-018; IT-016, IT-017
(verify-before-quiesce staging guarantee, N-003), IT-029, IT-030; UT-065–UT-070, UT-092–UT-095,
UT-100–UT-102, UT-108.

The one product action supplies restart consent; no second runtime/app confirmation is required.
Leaving the offer alone keeps work running. The packaged update regression must assert the new
daemon version and preserved home state, not only the new app marker. A compatible newer runtime
remains untouched and CLI output reports that actual version rather than substituting the app
release number. The historical separate timing-prompt branch is not a current acceptance step.

Per-OS release acceptance must confirm native installation, runtime readiness and preserved user
state on Linux and macOS. APP-runtime-update-managed owns the recommendation-only branch.

2026-10-21 targeted implementation verification: [update-flow report](../reports/2026-10-21-desktop-update-flow.md). The current full scenario is flagged
`untested`; native fixture and owning-suite results do not replace published-release or
cross-platform acceptance. Prior dated reports retain their historical evidence.

2026-10-27 update continuity regression: start concurrent work and a delegated child before
app Update. After native installation and runtime replacement, the same session IDs and partial
text remain; eligible parents and children continue without a manual prompt or a false failure.
The child completes and delivers its durable result once. An explicitly stopped control remains
stopped, and a subsequent restart does not duplicate continuation or child delivery. A failed
bundle install leaves the live daemon untouched; failure to request coordinated restart never
falls back to user-stop signaling. Existing opt-out, pending-interaction and binding-change gates
remain in force. E2E-016 owns the app/daemon handoff; session/daemon suites own recovery safety.
Evidence and execution limits: [continuity report](../reports/2026-10-27-update-session-continuity.md).

2026-11-04 large-telemetry upgrade (#1251): upgrade a v0.5.0 home whose global database still
holds a large `event_summary_export` (fixture of at least 1 GB, or a copy of a real long-running
install) through the desktop app's bundled-runtime replacement. While the replacement daemon
relocates telemetry, the boot window shows "Preparing CompozyOS" with "Upgrading data…" and an
advancing percentage instead of "CompozyOS could not start"; `desktop-bootstrap.jsonl` records
`provision` events with `stage: "migrate"` and rising `pct`, and
`~/.compozy/restarts/<operation>.json` carries `progress.phase: "migrating"` with an advancing
`completed` count. The upgrade ends in the product window with no wall-clock failure, and no
`~/.compozy/bin/.compozy-bootstrap-backup-*` copy remains. A relocation that stops advancing for
the idle window (5 minutes) fails with "daemon data upgrade … stopped progressing" and retains the
recovery copy; a relocation that exits fails as before. Telemetry rows and the event cursor match
the pre-upgrade database (lossless, resumable after interruption at any batch). Owning suites:
`TestGlobalDBTelemetryRelocation`, `TestRestartMigrationProgress`,
`TestDesktopRuntimeBootstrapReplacement`, desktop `bootstrap-state`/`bootstrap-event`. Lab
evidence (300k rows, ~700 MB, replacement boot with a restart journal): progress advanced every
~1s from 0 to 300000 rows over ~28s, then the journal turned `ready` with progress cleared.

2026-11-12 boot window redesign (#1549): the bundled-runtime replacement now reads "Updating CompozyOS" with the `0.5.4 → 0.6.0` pill and a working "What's new" link to the target release notes. The step list includes Upgrade data while the replacement migrates. Migrate events carry `completed`, `total` and `migration` (`telemetry_relocation`, `usage_cost_repair`, `session_history_upgrade`). The window shows `N of M sessions` or records, a percentage, and an estimate once the rate is known; it no longer shows the old "Upgrading data…" label. Owning suites: `TestDesktopRuntimeBootstrapReplacement` (counts and versions), `TestBootSessionRepair`, `TestGlobalDBUsageCostRepair`, desktop `boot-journey`. The long walk lives in APP-boot-long-data-upgrade. Status reset to untested.
