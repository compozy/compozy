---
id: APP-single-instance-focus
area: APP
title: Launching again focuses the one existing window
persona: Bruno
journey: J-desktop-link-driven
expected: With the app running (including minimized or on another virtual desktop), a second launch from dock/launcher/CLI focuses the existing window per platform convention, the process count stays unchanged, and a link argument on the second launch is forwarded, never dropped.
entry_points: second launch via dock/launcher/file manager; compozy app open while running
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status: blocked-verify
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-desktop-app-release-20260810-110811-513872-lab/qa-artifacts/qa/platform-capability-blockers.txt
last_report: docs/qa/reports/2026-08-10-desktop-app-release.md
overlaps:
---

PRD stories: US-009 (AC-1/AC-2 focus per convention; EC-1 link forwarded; EC-2 stale
single-instance state after crash recovers). BR-3. Test IDs: E2E-007; IT-020 (Linux/Windows
runner; macOS via E2E-007/E2E-023 smoke); UT-048, UT-082. During first-run provisioning a second
launch focuses and never starts a parallel provisioning (US-002.EC-1).

Per-OS evidence (N-004): all three OSes capture the focus/unminimize behavior with process-count
proof before/after the second launch, plus one second-launch-with-link forwarding walk. The
stale-lock crash recovery (EC-2) is walked on at least one scripted OS. macOS is scripted-manual
(screen recording + `ps` transcript).
