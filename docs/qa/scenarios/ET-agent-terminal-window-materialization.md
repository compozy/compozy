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

CI repair 2026-11-12 (#1565 integration): the existing terminal-agent E2E journey now
checks the on-demand contract introduced by #1570: no automatic agent window, explicit
opening while the process is held on stdin, removal after controlled exit, and retained
journal access from another live terminal. Approval navigation activates a hidden session
tab before asserting its permission dock; session close uses the canonical tab-aware helper.
The prior CI artifacts showed an obsolete automatic-window expectation and hidden-tab controls.
Validation remains CI-only under the operator's parallel-execution restriction.

CI journey alignment (#1547): `terminal-agent.spec.ts` E2E-003 asserts that an approved
agent exec does not create a desktop window or steal session focus, then explicitly
opens its exact terminal and checks output before releasing a stdin barrier. It
verifies successful exit and retained output through `terminal_wait`, expects the
agent terminal window to retire, then reopens Terminal to check journal provenance.
A fixed sleep must not race window opening against the intentional exit retirement.
E2E-005 restores the owning session before its approval when Terminal is the active
tab; E2E-008 closes only the conversation tab, keeping the terminal available for the
no-conversation selection fallback after selecting output on the remounted terminal. Existing approval, redaction and quote assertions
remain in the canonical suite. CI owns the real daemon/browser walk; no local run is claimed.
