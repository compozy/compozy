---
id: APP-web-update-two-track
area: APP
title: Read and apply both update tracks from Settings in a plain browser
persona: Bruno
journey: J-desktop-update-moment
expected: Settings → General → Updates shows one row per track the daemon reports, offers apply only where self-apply is possible, renders the daemon's named phases while an operation runs, and reports staged, blocked, and rolled-back outcomes without ever claiming success it cannot know.
entry_points: Settings → General → Updates in Chrome or the app; GET /api/settings/update; POST /api/settings/update/apply; POST /api/settings/update/cancel over HTTP and UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: APP-single-command-multi-target-update; APP-runtime-update-managed; APP-cancel-dormant-update; APP-web-update-indicator
---

Added 2026-08-16 for the Electron shell web update surface (ADR-006 S1). Task_07 owns the walk.

PRD stories: US-015, US-017, US-018, US-029 (AC-1 both tracks, AC-3 browser/app equivalence; EC-1
managed install, EC-2 apply from browser, EC-3 no app installed, EC-5 post-update truth). Test ids:
UT-040–UT-048, E2E-019, E2E-020.

Branches to walk, each against real daemon truth rather than a stubbed payload:

- both tracks available, runtime-only available, and both up to date;
- managed runtime (package-manager install) — the recommendation renders verbatim and **no apply
  control exists in the DOM**, not a disabled one;
- headless host with no desktop app — the app row is absent entirely, not an empty row;
- apply from a plain browser — progress renders the daemon's own phase names (`download`, `verify`,
  `install`, `start`, `ready-check`, `ready`) with percent only where measured, and the section
  polls at 2s while the operation is live;
- app closed — staged state with the daemon's next-launch message and a working cancel for the
  dormant operation;
- blocked — a second surface holds the lease, the holder is named, and no optimistic success shows;
- failed — the restored version and last error are reported, and the apply offer returns once the
  lease is free;
- refresh failure — the section header carries the failure once while the rows keep last-known truth.

Browser/app equivalence is part of the verdict: the same walk must render identically in the desktop
app and in a plain browser, because the SPA carries no desktop awareness.
