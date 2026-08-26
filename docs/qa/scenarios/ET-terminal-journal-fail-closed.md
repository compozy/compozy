---
id: ET-terminal-journal-fail-closed
area: ET
title: Refuse new terminal work when the command record cannot be written
persona: Rafa
journey: J-audit-terminal-work
expected: When the command record cannot be written, the terminal stops accepting new input and new commands with a stated audit reason while existing output stays readable and watchable, and normal operation resumes by itself once the pending records land — no command ever runs unrecorded.
entry_points: Terminal app while the record store is failing; compozy terminal exec; compozy terminal respond; compozy terminal journal; terminal input answer route; global, workspace, and profile config.toml shell_integration
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-terminal-journal-recording; ET-terminal-redaction-boundaries
---

Planned by integrated-terminal task 09 for the fail-closed audit posture.
`ET-terminal-journal-recording` owns reading and filtering a healthy journal; this file owns what
happens when the record cannot be written, and how honestly the record describes itself. Task 10 owns
the walk, evidence, and verdict.

Walk:

1. With a terminal running, make the command record store fail, then try to run a new command and to
   send new input; confirm both are refused with an audit reason that says the record is the blocker.
2. Confirm that during the same window the terminal's existing output stays readable, watchers stay
   attached, and nothing already running is killed.
3. Restore the store and confirm the pending records land and the terminal starts accepting work again
   without an operator having to restart anything.
4. Confirm the journal afterwards contains every command that actually ran, with no gap and no
   duplicate row for the blocked attempts.
5. Turn the command-boundary marking off in configuration and confirm the journal keeps working with
   its boundaries honestly labelled as approximate rather than presented as exact.
6. Run one command whose boundary is detected exactly and one that is only approximated, and confirm
   the record distinguishes them on both the browser and the command-line read.
7. Confirm every row names the actor who ran it, the approval that authorised it, and the profile it
   ran under.
