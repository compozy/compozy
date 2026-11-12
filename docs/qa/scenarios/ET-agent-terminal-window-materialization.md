---
id: ET-agent-terminal-window-materialization
area: ET
title: Open agent terminals on demand without stale desktop windows
persona: Marina
journey: J-operate-integrated-terminal
expected: Agent terminals stay in the catalog until explicitly opened; exit or pruning removes their windows and restore history; startup retires stale terminal bindings; the Terminal rail only marks live terminals.
entry_points: Session composer; compozy__terminal_open; compozy__terminal_exec with visible true; Web desktop window deck; Web dock Terminal app
qa_status: partial
bug_ids:
fix_status:
retest_status: partial
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-terminal-rework-20260901-150952-749450-lab/qa-artifacts/qa;docs/qa/reports/2026-09-01-terminal-rework.md;/Users/pedronauck/dev/qa-labs/compozy-skill-terminal-recovery-20260902-205559-939126-lab/qa-artifacts/qa/test-cases/terminal-prompt-replay.md;dev-session:sess-795e2e3bb8afc603
last_report: docs/qa/reports/2026-09-02-skill-terminal-recovery.md
overlaps: ET-terminal-session-block-handoff; ET-terminal-window-native-flow
---

Agent terminals remain outside the desktop layout until the operator opens one.

Walk in an isolated home, never against the operator's existing windows:

1. Open several agent PTY terminals in one session. Confirm no desktop windows appear and the live
   catalog remains available from Terminal.
2. Explicitly open one terminal. Confirm live output streams, then exit it and confirm its window
   disappears. Undo and Reopen must not restore the dead terminal.
3. Prune a retained terminal and confirm no desktop references remain.
4. Persist a terminal window, restart only the isolated daemon, and confirm stale terminal windows
   disappear across project, All projects and User desktops while other apps remain.
5. Confirm the rail is idle for an exited/missing terminal even if a stale minimized window exists;
   a running terminal lights the rail even with no window open.

Targeted validation: daemon bridge event tests; real SQLite partition cleanup and real PTY lifecycle
in the owning daemon runtime suite; window-manager topology/history suite; rendered dock regression
suite and Storybook terminal rail captures. Full E2E remains CI-owned.
