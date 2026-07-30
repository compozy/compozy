---
id: ET-web-session-transcript-calm-grammar
area: ET
title: Session transcript renders the calm-surface grammar
persona: Théo
journey: J-14
expected: Settled tool runs rest as one semantic summary line ("Ran N commands · Edited N files", distinct-file counts) that expands to plain 24px tool rows; the live tail shows the last 4 calls behind a "+N previous tool calls" toggle; failed calls stay individually visible and collapsed with the red × glyph and error-first-line preview (row text never turns red, success check is grey); settled turns fold behind "Worked for Ns" (the only border), interrupted turns never fold; runtime events render as one-line markers (kind as mono meta, consecutive same-kind ×N), never tinted Alert cards; the user message is a borderless 4.5% ink bubble clamping at 176px with "Show more"; TodoWrite renders as a plan list, never JSON; changed files render as an "Edited N files +A −D" line expanding to bare mono file lines (cap 8).
entry_points: web session window transcript; session transcript REST + SSE
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: RT-session-message-reload, ET-tool-result-artifact-recovery, ET-web-session-thread-full-bleed
---

story: As a person supervising agent work I read a calm, text-first transcript where settled work collapses to semantic summaries and only failures and the live tail demand attention.

errors:

inventory: Needs QA — introduced by the session transcript redesign (calm-surface vocabulary, 2026-07-29).
