---
id: ET-web-session-transcript-calm-grammar
area: ET
title: Session transcript renders the calm-surface grammar
persona: Théo
journey: J-14
expected: Settled tool runs rest as one semantic summary line ("Ran N commands · Edited N files", distinct-file counts) that expands to plain 24px tool rows; the live tail shows the last 4 calls behind a "+N previous tool calls" toggle; failed calls stay individually visible and collapsed with the red × glyph and error-first-line preview (row text never turns red, success check is grey); settled turns fold behind "Worked for Ns" (the only border), interrupted turns never fold; normal queue, steer, interrupt, and dropped-input lifecycle markers do not render as warning noise; the active turn has one working row immediately above the composer; runtime events render as calm one-line markers, never tinted Alert cards; the user message, TodoWrite plan, and changed-file roll-up retain their compact semantic treatments.
entry_points: web session window transcript; session transcript REST + SSE
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-03-session-input-controls/02-steer-settled-calm.png; docs/qa/evidence/2026-08-03-session-input-controls/04-interrupt-retest-calm.png; docs/qa/evidence/2026-08-03-session-input-controls/storybook/busy-input-controls.png
last_report: docs/qa/reports/2026-08-03-session-input-controls.md
overlaps: RT-session-message-reload, ET-tool-result-artifact-recovery, ET-web-session-thread-full-bleed
---

story: As a person supervising agent work I read a calm, text-first transcript where settled work collapses to semantic summaries and only failures and the live tail demand attention.

errors:

inventory: Needs QA — introduced by the session transcript redesign (calm-surface vocabulary, 2026-07-29).

QA impact 2026-08-03: reset for calm busy-input feedback and the single active-turn working row.

QA pass 2026-08-03: lifecycle markers stayed durable but invisible as warning noise; expected cancellation no longer projected provider failure; active turns rendered exactly one working row above the composer.
