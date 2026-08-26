---
id: ET-agent-reported-terminal
area: ET
title: Distinguish agent-reported output from supervised terminals
persona: Marina
journey: J-operate-integrated-terminal
expected: Agent-internal command output appears in the session with a clear reported-by-agent label and no terminal controls, while the Terminal app, journal, and window list stay empty.
entry_points: Session transcript; Web dock Terminal app; terminal hook observer
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Flagged by integrated-terminal task 07. Task 10 owns the real-user walk, evidence, and verdict.

Walk:

1. Start the deterministic agent-reported terminal fixture from a session.
2. Confirm the transcript block is labeled `reported by agent`, renders the reported command output, and offers no typing, takeover, open, record, or kill controls.
3. Open the Terminal app and confirm its terminal list and journal remain empty.
4. Confirm no `terminal.*` hook event was emitted for the reported-only output.
5. Confirm no Terminal window was created and a session turn with no report renders no terminal block.
