---
id: ET-palette-agent-fallback
area: ET
title: Delegate an unmatched palette query to the default agent
persona: Bruno
journey: J-command-os-from-palette
expected: A non-empty query with no matching result shows only the visually distinct Ask agent row, while a weak result at the served threshold keeps both the result and fallback visible. Nothing sends the query before Enter. Enter creates one new session with the workspace default agent and the query as its opening prompt, closes the palette, and opens that session; a missing default opens the agent picker, a failed spawn preserves the query and names the failure, and a rapid repeated Enter cannot create duplicate sessions. Turning Agent fallback off in Settings > Palette removes the row immediately and `fallback_targets = []` reports the same desired state.
entry_points: Command-K; Ask agent result row; Settings > Palette; `compozy config get cmd_palette.fallback_targets`; GET|PATCH /api/settings/cmd-palette
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-palette-registry-driven-root; ET-agent-command-invoke; ET-web-command-palette-shortcuts; ET-palette-sessions-view-switch
---

Flagged by command-palette task 10. Task 12 owns the first isolated real-user walk, E2E-013,
visual-contract comparison, and verdict.

The fallback ⏎ lands through the same shared session-landing path (BR-20) the Sessions view uses —
the landing half is canaried by `ET-palette-sessions-view-switch`; this walk owns the row assembly,
threshold, transmission, and settings lifecycle.

Walk (task_11 plan):

1. Type gibberish — only the visually distinct "Ask agent: '{query}'" row renders; an empty query
   renders no fallback row.
2. Type a weak-but-nonzero match — the result and the fallback row render together (threshold
   behavior: at the served threshold both; below it fallback-only).
3. Watch the network while typing — nothing carries the query before ⏎.
4. Press ⏎ — one new session opens with the workspace default agent and the query as its opening
   prompt; the palette closes; a rapid double-⏎ creates exactly one session.
5. Remove the workspace default agent and repeat — ⏎ opens the agent picker first, then proceeds.
6. Break session spawn (provider down) — the failure toast names the reason and the palette reopens
   with the query preserved.
7. Toggle Agent fallback off in Settings > Palette — the row disappears immediately; confirm
   `compozy config get cmd_palette.fallback_targets` reports `[]`; re-enable restores it.

Expected evidence: screenshots of the zero-match and weak-match states, the created session with
the query as first prompt, the picker path, and the Settings toggle; a network capture (or devtools
note) proving no pre-send; the config-get transcript.
