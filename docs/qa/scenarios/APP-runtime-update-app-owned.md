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
