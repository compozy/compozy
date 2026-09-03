---
id: ET-terminal-session-block-handoff
area: ET
title: Follow a deliberate terminal run from the session transcript
persona: Bruno
journey: J-operate-integrated-terminal
expected: A deliberate terminal tool call renders a live block in the transcript with its exit outcome, jumps to the owning terminal window, hands a still-running command over instead of pretending it finished, and marks the output it feeds back to the model as untrusted data.
entry_points: Session transcript terminal block; Open terminal affordance; Web dock Terminal app; compozy__terminal_exec with visible true; compozy__terminal_read; compozy__terminal_wait
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-terminal-rework-20260901-150952-749450-lab/qa-artifacts/qa; docs/qa/reports/2026-09-01-terminal-rework.md
last_report: docs/qa/reports/2026-09-01-terminal-rework.md
overlaps: ET-terminal-browser-lifecycle; ET-agent-terminal-window-materialization
---

Planned by integrated-terminal task 09 for the deliberate-use transcript surface (`_uiux.md` S3);
this file owns the supervised block and its handoff. Reset by ADR-019 (S4 removed; agent-opened
terminals materialize windows) and re-walked on 2026-09-01 — see `last_report`.

Walk:

1. Ask an agent to run a short visible command in a terminal and watch the transcript block stream its
   output while the command is running.
2. Let the command finish and confirm the block shows the exit outcome, including a non-zero exit
   rendered as a plain code rather than an alarm.
3. Use the block's open affordance and confirm it focuses the terminal window the daemon already
   materialized for this run rather than opening a second one.
4. Start a command that outlives the yield window and confirm the transcript hands over the running
   terminal's identity instead of claiming completion.
5. Read the same output back through the agent's read and wait paths and confirm every model-facing
   result carries the untrusted-data marking.
6. Confirm the supervised block carries the controller identity and watch state, and that
   agent-internal commands in the same transcript render as plain command output with no terminal
   chrome.
