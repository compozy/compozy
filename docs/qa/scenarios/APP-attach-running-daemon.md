---
id: APP-attach-running-daemon
area: APP
title: Attach to my already-running runtime and touch nothing
persona: Dora
journey: J-desktop-attach-daily
expected: With a healthy runtime and an active session, the app shows the identical workspace/session state the browser tab shows (same origin, same local UI state), no second daemon appears, the existing process is never terminated, and side-by-side actions reflect live in both surfaces.
entry_points: dock/launcher icon with a running daemon; browser tab open side by side
qa_status: untested
bug_ids: BUG-20260810-desktop-runtime-stalls
fix_status: fixed
retest_status: blocked-verify
fix_commits: b415f24b; b3aa3d27; bd610cfa; 02b55a46
evidence: /Users/pedronauck/dev/qa-labs/compozy-coderabbit-desktop-remediation-20260810-153824-470714-lab/qa-artifacts/qa/app-attached.jpeg; /Users/pedronauck/dev/qa-labs/compozy-coderabbit-desktop-remediation-20260810-153824-470714-lab/qa-artifacts/qa/quit-runtime-survives.json; /Users/pedronauck/dev/qa-labs/compozy-coderabbit-desktop-remediation-20260810-153824-470714-lab/qa-artifacts/qa/platform-capability-blockers.txt; /Users/pedronauck/dev/qa-labs/compozy-desktop-startup-diagnostics-20260811-200336-439901-lab/qa-artifacts/qa/runtime-cli-walk.md
last_report: docs/qa/reports/2026-08-11-desktop-startup-diagnostics.md
overlaps:
---

PRD stories: US-003 (attach, zero writes; AC-3 isolated home; EC-1 stale record; EC-2 foreign
squatter; EC-3 unhealthy), US-020 (browser coexistence AC-1/AC-2, EC-1 different homes never
mixed). Test IDs: E2E-003, E2E-018; IT-001, IT-004, IT-026, IT-027, IT-028; UT-013–UT-023,
UT-089–UT-091.

Per-OS evidence: macOS and Linux capture process-table before/after app open (no spawn),
same-origin proof (session + local UI state parity with the tab), and the live two-way sync walk
(E2E-011). Both use Playwright `_electron` plus the recorded manual sync check.
Isolated-home labs only: the lab manifest's `COMPOZY_HOME` must be the one resolved — never the
operator default home.

QA impact 2026-08-11: the startup resolver now waits for a live recorded daemon and restricts any
automatic repair to verified desktop-owned processes. The isolated macOS walk reopened the app,
kept daemon PID `89043`, and retained one listener. Browser/session parity and the shipping OS
matrix remain blocked for artifact verification.
