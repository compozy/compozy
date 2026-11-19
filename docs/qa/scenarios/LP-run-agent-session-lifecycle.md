---
id: LP-run-agent-session-lifecycle
area: LP
title: Stop terminal run-agent child sessions
persona: Bruno
journey: J-complete-partial-loop
expected: A session-started Loop creates each run-agent worker as a run-owned system child of the nearest origin session, stops it after successful, failed, or canceled terminal settlement, and keeps it active only while a retry can resume the same cell.
entry_points: compozy__loop_run; compozy loop status; compozy session status; session catalog HTTP/UDS/CLI
qa_status: pass
bug_ids: compozy/compozy#1851
fix_status: fixed
retest_status: pending
fix_commits: 6ea6dbd4b
evidence: docs/qa/reports/2026-08-20-run-agent-session-lifecycle.md; docs/qa/reports/2026-08-20-run-agent-terminal-cleanup.md
last_report: docs/qa/reports/2026-08-20-run-agent-terminal-cleanup.md
overlaps: LP-run-agent-output-ownership; RT-loop-goal-origin-session-lineage; LP-crash-death-resume
---

Added for GitHub issues #444 and #445. The acceptance walk starts a nested Loop from a real
parent session, captures the `run-agent` worker session while it is active, and confirms its
`parent_session_id` and `root_session_id` point to the nearest originating session without borrowing
that session. After successful, failed, or canceled terminal settlement, fresh structured reads
must show the worker stopped and unbound while the parent session remains active. A retry probe must
keep the exact worker binding active until the cell reaches a true terminal boundary.

For #1851, use a Loop that allows `native_tool` starts and has an isolated `run-agent` node. Start
it with `compozy__loop_run` from an always-on main and repeat from an on-demand session created by
that main. In both runs, the node must complete and persist its output. The worker is a distinct
system session in the same workspace and profile, with the calling session as its parent and the
main as its root. Give the selected worker agent a tool outside the instance ceiling and confirm
the worker cannot use it. The managed Loop runtime integration suite executes `run-agent` with a
real cell binding and checks creation and binding reuse for both origins. The live-provider walk
remains pending.
