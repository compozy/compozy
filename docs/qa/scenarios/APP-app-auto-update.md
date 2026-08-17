---
id: APP-app-auto-update
area: APP
title: Apply an app update through the product update surface
persona: Bruno
journey: J-desktop-update-moment
expected: With a newer verified release, the daemon reports the app track in Settings and the menubar points there; an accepted durable operation downloads and verifies the asset, then a running app enters installer handoff only with consent while a closed app stages for next launch. The boot window reports progress but never offers an update.
entry_points: Settings → General → Updates in Chrome or the app; product menubar update indicator; compozy update; signed release channel
qa_status: untested
bug_ids:
fix_status:
retest_status: blocked-verify
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-desktop-app-release-20260810-110811-513872-lab/qa-artifacts/qa/platform-capability-blockers.txt
last_report: docs/qa/reports/2026-08-10-desktop-app-release.md
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
