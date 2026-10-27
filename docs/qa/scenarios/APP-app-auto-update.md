---
id: APP-app-auto-update
area: APP
title: Apply an app update through the product update surface
persona: Bruno
journey: J-desktop-update-moment
expected: One Update activation downloads and verifies the release, waits for native installer readiness while the app remains usable, then restarts on the new version. A desktop-owned runtime updates from that app bundle; a closed app stages for next launch. The boot window reports progress but never offers an update.
entry_points: Settings → General → Updates in Chrome or the app; product menubar update indicator; compozy update; signed release channel
qa_status: untested
bug_ids: BUG-20260817-macos-channel-manifest-includes-dmg; BUG-20260817-linux-channel-manifest-includes-deb
fix_status: fixed
retest_status: pending
fix_commits: 361c785e; 437cea3d
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-08-17-electron-shell.md
overlaps:
---

PRD stories: US-014 (AC-1 background check/download, AC-2 consent restart, AC-3 unverifiable never
applied, AC-4 silent no-op; EC-1 offline skip, EC-2 interrupted download, EC-3 pending applied
next launch, EC-5 sleep/wake). BR-6/BR-7/BR-10. Test IDs: E2E-016, E2E-020; IT-013, IT-014,
IT-015, IT-025; UT-056–UT-062, UT-064, UT-114, UT-115.

Per-OS evidence: E2E-016/E2E-020 run on Linux and macOS against the mock GitHub Release and
channel-beta fixture, including the macOS backup posture. Both release OSes record before/after
version indicators and the closed-app staged-update walk. Sleep/wake is a per-OS platform-smoke item with a
recorded resume-without-duplicate-prompt observation.

QA impact 2026-08-16: the decision surface moved from the shell overlay to the daemon-backed web UI
and app apply joined the durable multi-target operation. Reset for the Electron N→N+1 walk.


Journal-consumer regression (issue #804): stage the app after the runtime reaches `finalized`.
The desktop must parse the daemon journal's `swapping`, `health-checking`, and `finalized` phases,
and consume the staged app after runtime readiness. An archived failed install of an older app
release must not suppress the newly discovered release or replace its version with the failed one.

2026-10-21 regression: native `update-downloaded` must precede product-window disposal; ordinary
`window-all-closed` quitting must not win over installer handoff. E2E-016 uses different app and
bundled-runtime versions and verifies the running daemon after relaunch. On Rosetta, the signed
app plan and updater must select the same native-arm64 artifact without relaxing digest checks.

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
