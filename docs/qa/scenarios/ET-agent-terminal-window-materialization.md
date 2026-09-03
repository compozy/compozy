---
id: ET-agent-terminal-window-materialization
area: ET
title: Watch an agent-opened terminal materialize as a desktop window
persona: Marina
journey: J-operate-integrated-terminal
expected: When an agent opens an interactive terminal (terminal_open, or terminal_exec with visible true), a Terminal window for that exact terminal appears on the desktop without stealing focus; closing the window leaves the process running and the window does not reopen; agent-internal commands never materialize anything.
entry_points: Session composer; compozy__terminal_open; compozy__terminal_exec with visible true; Web desktop window deck; Web dock Terminal app
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-terminal-rework-20260901-150952-749450-lab/qa-artifacts/qa;docs/qa/reports/2026-09-01-terminal-rework.md;/Users/pedronauck/dev/qa-labs/compozy-skill-terminal-recovery-20260902-205559-939126-lab/qa-artifacts/qa/test-cases/terminal-prompt-replay.md;dev-session:sess-795e2e3bb8afc603
last_report: docs/qa/reports/2026-09-02-skill-terminal-recovery.md
overlaps: ET-terminal-session-block-handoff; ET-terminal-window-native-flow
---

Added by ADR-019: agent-opened interactive PTY terminals materialize as managed Terminal windows in
the daemon window model, one-shot per terminal, unfocused, on the desktop showing the bound session
when one exists.

Walk:

1. In a session, ask the agent to open a terminal and start a watchable process (for example a dev
   server) using its terminal tools.
2. Confirm a Terminal window for that exact terminal id appears on the desktop without moving
   keyboard focus away from the session, and that the window streams the live output.
3. Confirm the window landed on the desktop that shows this session's window.
4. Close the Terminal window and confirm the process keeps running (catalog and dock still show it)
   and the window does not reopen on its own.
5. Ask the agent to run several routine internal commands and confirm they render as plain command
   output in the transcript — no terminal blocks, no windows, no catalog entries.
6. Ask the agent to run a hidden pipe execution that outlives the yield window and confirm it lists
   as a pipe terminal without materializing a window.

QA impact 2026-09-02: reset because the official Compozy skill now permits a bounded descriptor-based
recovery after repeated reference-read failures. Re-walk the original ordinary-language prompt and
confirm the agent reaches a visible terminal instead of stopping on the missing-reference branch.
Charter: `CH-terminal-reference-recovery`.

Retest 2026-09-02: pass. The exact `/agents:compozy` prompt loaded the current bundled terminal
reference despite a stale physical workspace copy, opened `term-2b7ed0bb80ea`, ran two safe command
sequences, waited for idle, and yielded the visible terminal to the operator.
