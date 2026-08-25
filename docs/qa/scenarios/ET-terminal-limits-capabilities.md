---
id: ET-terminal-limits-capabilities
area: ET
title: Explain terminal limits and platform capabilities honestly
persona: Dora
journey: J-administer-terminal-capacity
expected: Workspace and viewer caps identify the blocking limit and recovery action, while a sandbox workspace exposes execute-only output without interactive controls or claims.
entry_points: Terminal app; Settings terminal section; sandbox workspace; structured terminal surfaces
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-terminal-config-lifecycle
---

Flagged by integrated-terminal task 06. Task 10 owns the real-user walk, evidence, and verdict.

Walk:

1. Reach the workspace terminal cap and verify the dialog names existing terminal IDs and Settings.
2. Reach the subscriber cap from another viewer and verify the refusal is specific.
3. Open Terminal in a sandbox workspace and verify command output remains available in execute-only mode.
4. Confirm interactive controls and interactive capability claims are absent in that workspace.

