---
id: ET-agent-reported-terminal
area: ET
title: Distinguish agent-reported output from supervised terminals
persona: Marina
journey: J-operate-integrated-terminal
expected: Agent-internal command output appears in the session with a clear reported-by-agent label and no terminal controls, while the Terminal app, journal, and window list stay empty.
entry_points: Session transcript; Web dock Terminal app; terminal hook observer
qa_status: pass
bug_ids: BUG-20260826-terminal-journal-workspace-id
fix_status: fixed
retest_status: pass
fix_commits: b745ebcbcfe6
evidence: /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/test-e2e-runtime-after-fix.log; docs/qa/reports/2026-08-26-integrated-terminal.md
last_report: docs/qa/reports/2026-08-26-integrated-terminal.md
overlaps:
---

Flagged by integrated-terminal task 07. Task 10 owns the real-user walk, evidence, and verdict.

Walk:

1. Start the deterministic agent-reported terminal fixture from a session.
2. Confirm the transcript block is labeled `reported by agent`, renders the reported command output, and offers no typing, takeover, open, record, or kill controls.
3. Open the Terminal app and confirm its terminal list and journal remain empty.
4. Confirm no `terminal.*` hook event was emitted for the reported-only output.
5. Confirm no Terminal window was created and a session turn with no report renders no terminal block.
