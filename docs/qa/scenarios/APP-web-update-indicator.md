---
id: APP-web-update-indicator
area: APP
title: Update CompozyOS in one click from the menubar
persona: Bruno
journey: J-desktop-update-moment
expected: One Update activation by pointer or keyboard updates CompozyOS in place. The pill holds a disabled Updating spinner in the same accent ink as the Update offer while the operation runs, and the app returns on the new version. A settled app failure offers Retry update directly; a failure without an eligible retry opens Settings. A package-manager install opens Settings with its upgrade command.
entry_points: product menubar in Chrome or the app; keyboard focus with Enter or Space; GET /api/settings/update; POST /api/settings/update/apply
qa_status: untested
bug_ids:
fix_status: 
retest_status: 
fix_commits: 
evidence: 
last_report: docs/qa/reports/2026-08-17-electron-shell.md
overlaps: APP-web-update-two-track; APP-app-auto-update
---

Added 2026-08-16 for the Electron shell web update surface (ADR-006 S2). Reworked 2026-10-12 to a
one-click update, modeled on how the ChatGPT desktop app updates: no Settings detour, and the
runtime/app split never reaches the user. Re-walk required.

Test ids: `menubar-update-indicator.test.tsx` owns the per-state rendering and callbacks;
`update-presentation.test.ts` owns the snapshot → state mapping (`settingsUpdateIndicatorState`).
Combined E2E-021 / E2E-022 in `os-shell.spec.ts` owns keyboard and pointer activation, applying
without opening Settings.

Branches to walk:

- nothing offered: the pill is **absent from the DOM**, with no focusable control left in the
  tab order;
- one part available, then both: the same single "Update" pill either way, with no count, no
  badge, and no dropdown;
- click Update in the desktop app: the pill switches to "Updating…" immediately, keeping the
  offer's accent ink on its spinner and label in light and dark (never grey), Settings never
  opens, the app quits and relaunches by itself, and About / Settings show the new version;
- click Update in a plain browser with no desktop app: the pill shows "Updating…" while the daemon
  restarts, and the tab reloads by itself onto the new version;
- staged (the app is closed while a browser applies): the pill disappears; Settings explains that
  the update finishes on next launch;
- failed app with an eligible release and no active operation: the pill reads "Retry update" in
  the danger tone and starts a freshly validated attempt without opening Settings;
- failed without an eligible retry: "Update failed" opens Settings → General with the error;
- package-manager install: "Update" opens Settings, which shows the upgrade command;
- keyboard-only: the pill is reachable by Tab, shows a visible focus ring against the menubar
  glass, and activates with Enter or Space.

Walk this in a plain browser as well as the desktop app. The pill is fed by daemon truth, so
both must behave the same.

2026-10-21 targeted implementation verification: [update-flow report](../reports/2026-10-21-desktop-update-flow.md). The current full scenario is flagged
`untested`; native fixture and owning-suite results do not replace published-release or
cross-platform acceptance. Prior dated reports retain their historical evidence.

2026-10-31 (#1087): the Updating pill lost its accent ink to the shared disabled-button grey.
Verified on the rendered `UpdateIndicatorAvailable` / `Updating` / `Failed` stories in light and
dark (computed ink matches the offer); the full scenario walk remains `untested`.
