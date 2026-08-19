---
id: ET-palette-inline-args-confirmation
area: ET
title: Supply command arguments and confirm destructive execution
persona: Bruno
journey: J-operate-command-palette
expected: An argument-bearing command replaces palette search with its declared text, password, and dropdown fields. Tab follows field order, invalid or missing values block execution and focus the first failing field, password values stay masked and leave no history or personalization trace, and Escape discards every value and restores search. A declared confirmation names the effect with Cancel focused, ignores the triggering key repeat, refuses an invalidated target, and hands successful or failed asynchronous execution to truthful pending and toast feedback with Retry only when the command is safe to repeat.
entry_points: command palette command row; bound command shortcut; command palette action panel
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-palette-action-panel; ET-palette-registry-driven-root; ET-agent-command-invoke
---

Flagged by command-palette task 04. Task 12 owns the first real-user walk, E2E-014, visual-contract
comparison, and verdict.
