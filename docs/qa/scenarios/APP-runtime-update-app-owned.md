---
id: APP-runtime-update-app-owned
area: APP
title: An app-provisioned runtime updates as one product update with timing consent
persona: Bruno
journey: J-desktop-update-moment
expected: On an app-provisioned home with agent work in flight, a ready runtime update asks for timing consent — "later" keeps everything working; "now" quiesces, stops, applies, restarts, reconnects — and both new versions appear in one update surface.
entry_points: in-app update surface on an app-provisioned home; signed runtime.json feed
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status: blocked-verify
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-desktop-app-release-20260810-110811-513872-lab/qa-artifacts/qa/platform-capability-blockers.txt
last_report: docs/qa/reports/2026-08-10-desktop-app-release.md
overlaps: APP-runtime-update-managed
---

PRD stories: US-016 (AC-1 one experience, AC-2 timing consent under in-flight work, AC-3
reconnect + new version; EC-1 fail → previous usable + diagnostics; EC-2 repeated decline, no
nagging; EC-3 ownership flip → managed behavior). BR-8/BR-9. Test IDs: E2E-014; IT-016, IT-017
(verify-before-quiesce staging guarantee, N-003), IT-029, IT-030; UT-065–UT-070, UT-092–UT-095,
UT-100–UT-102, UT-108.

Per-OS evidence (N-004): the full consent → quiesce → swap → reconnect walk runs scripted on
Linux and Windows with the drain-readout transcript and both version indicators captured; macOS
walks it in the scripted-manual smoke. The "later keeps working" branch and the decline-no-nagging
posture are recorded on at least one OS. Overlap: APP-runtime-update-managed owns the
recommendation-only branch of the same update surface.
