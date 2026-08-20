---
id: ET-palette-agent-fallback
area: ET
title: Delegate an unmatched palette query to the default agent
persona: Bruno
journey: J-operate-command-palette
expected: A non-empty query with no matching result shows only the visually distinct Ask agent row, while a weak result at the served threshold keeps both the result and fallback visible. Nothing sends the query before Enter. Enter creates one new session with the workspace default agent and the query as its opening prompt, closes the palette, and opens that session; a missing default opens the agent picker, a failed spawn preserves the query and names the failure, and a rapid repeated Enter cannot create duplicate sessions. Turning Agent fallback off in Settings > Palette removes the row immediately and `fallback_targets = []` reports the same desired state.
entry_points: Command-K; Ask agent result row; Settings > Palette; `compozy config get cmd_palette.fallback_targets`
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-palette-registry-driven-root; ET-agent-command-invoke; ET-web-command-palette-shortcuts
---

Flagged by command-palette task 10. Task 12 owns the first isolated real-user walk, E2E-013,
visual-contract comparison, and verdict.
