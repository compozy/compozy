---
id: ET-session-composer-skill-chip
area: ET
title: Skill commands render as inline chips in the composer
persona: Théo
journey: J-use-session-slash-commands
expected: Selecting a skill from the slash menu renders it inside the composer as an inline chip (icon plus Title Case label, info tone, no fill) followed by a space, with the caret continuing after it; the outgoing prompt text keeps the exact canonical token; one Backspace over the chip removes the whole token; arrow keys jump over the chip as a unit; a restored draft containing a known skill token re-materializes the chip; built-in and agent commands stay plain text.
entry_points: web session composer; web session deep link
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-session-slash-commands-inline;ET-web-session-composer-text-entry
---

QA impact 2026-08-05: new user-visible behavior from the composer redesign — inline skill chips in the prompt editor (Lexical), mirroring the transcript's verified skill pills. The wire contract is unchanged: the daemon still receives the raw token text and returns skill_invocations; the chip is presentation only. Walk chip insertion at start and mid-text, atomic deletion, caret traversal, draft restore re-materialization, and confirm the transcript echo matches the sent token.
