---
id: ET-terminal-limits-capabilities
area: ET
title: Explain terminal limits and platform capabilities honestly
persona: Dora
journey: J-administer-terminal-capacity
expected: Workspace and viewer caps identify the blocking limit and recovery action. Terminal capability reads report the supported interactive and execution controls truthfully, and unsupported operations fail with a structured capability error.
entry_points: Terminal app; Settings terminal section; structured terminal surfaces
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/test-e2e-runtime-after-fix.log; docs/qa/reports/2026-08-26-integrated-terminal.md
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: MS-terminal-config-lifecycle
---

Workspace and viewer caps identify the blocking limit and recovery action. Terminal capability reads report the supported interactive and execution controls truthfully, and unsupported operations fail with a structured capability error.

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

QA impact 2026-09-28 (normie pass): Settings › Terminal limits (terminals per project/installation, viewers per terminal) moved into an "Advanced — limits" fold; open it before editing. Flag only.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

QA impact 2026-11-13 (issue #1659): the project-cap dialog no longer shows the raw
`terminal_limit_reached · terminal.max_per_workspace N` footer line; its header copy, Open Settings,
and Close stay. A Terminal window opened at the cap waits for a slot: when the running count drops
below the limit — the dialog's Close, another window, or `compozy terminal close` — the dialog
closes and the window opens a live terminal without a reload. Dismissing the dialog (Esc or outside
click) without closing a terminal ends the wait and shows the "No terminals yet" empty state with
its Open a terminal control instead of an endless "Opening a terminal". Walk both branches at the
cap with the next pass; covered meanwhile by the terminal window component suite.
