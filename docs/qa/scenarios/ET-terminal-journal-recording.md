---
id: ET-terminal-journal-recording
area: ET
title: Inspect command history and recordings
persona: Dora
journey: J-audit-terminal-work
expected: Journal filters change the server query, approximate boundaries are labeled, filtered misses are distinct from an empty history, and a recording replays from its owning profile with its retention stated.
entry_points: Terminal Journal tab; terminal journal CLI; terminal recording download; terminal selection to active session composer
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Flagged by integrated-terminal task 06. Task 10 owns the real-user walk, evidence, and verdict.

Walk:

1. Run exact and estimated commands, then open the Journal tab.
2. Filter by actor, time, terminal, and failed result; clear one filter and then all filters.
3. Verify an empty filtered result does not claim the project has no history.
4. Replay a recording and return to its journal row; compare the CLI and browser record.
5. Select a bounded terminal range and send it to the active session composer; verify the `<terminal_context>` source terminal, line range, and untrusted marker survive sending.
