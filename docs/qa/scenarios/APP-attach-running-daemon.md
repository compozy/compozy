---
id: APP-attach-running-daemon
area: APP
title: Attach to my already-running runtime and touch nothing
persona: Dora
journey: J-desktop-attach-daily
expected: With a healthy runtime and an active session, the app shows the identical workspace/session state the browser tab shows (same origin, same local UI state), no second daemon appears, and side-by-side actions reflect live in both surfaces.
entry_points: dock/launcher icon with a running daemon; browser tab open side by side
qa_status: blocked-verify
bug_ids: BUG-20260810-desktop-runtime-stalls
fix_status: fixed
retest_status: blocked-verify
fix_commits: b415f24b; b3aa3d27; bd610cfa; 02b55a46
evidence: /Users/pedronauck/dev/qa-labs/compozy-desktop-app-release-20260810-110811-513872-lab/qa-artifacts/qa/app-control-product.txt
last_report: docs/qa/reports/2026-08-10-desktop-app-release.md
overlaps:
---

PRD stories: US-003 (attach, zero writes; AC-3 isolated home; EC-1 stale record; EC-2 foreign
squatter; EC-3 unhealthy), US-020 (browser coexistence AC-1/AC-2, EC-1 different homes never
mixed). Test IDs: E2E-003, E2E-018; IT-001, IT-004, IT-026, IT-027, IT-028; UT-013–UT-023,
UT-089–UT-091.

Per-OS evidence (N-004): all three OSes capture process-table before/after app open (no spawn),
same-origin proof (session + local UI state parity with the tab), and the live two-way sync walk
(E2E-018). macOS via scripted-manual smoke; Windows/Linux via tauri-driver plus manual sync check.
Isolated-home labs only: the lab manifest's `COMPOZY_HOME` must be the one resolved — never the
operator default home.
