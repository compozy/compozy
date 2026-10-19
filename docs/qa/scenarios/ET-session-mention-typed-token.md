---
id: ET-session-mention-typed-token
area: ET
title: Typed mention tokens resolve from the CLI, pasted text, and agent-origin compozy__session_prompt
persona: Ada
journey:
expected: A pasted `@README.md` in the composer and `compozy session prompt <id> "Review @README.md"` deliver the same resource_link as the picker; an agent in session A calling compozy__session_prompt to session B with `@README.md` and `@session:<C>` makes B's agent receive a link to B's own README.md and the references block for C, even though agent-origin prompts cannot run slash commands; tokens in inline code or fenced blocks, trailing punctuation, and an unresolvable `@scope/pkg` behave as documented; over-cap tokens stay text.
entry_points: compozy session prompt; compozy__session_prompt; web session window composer (paste); compozy session history --json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-session-mention-file
---

E2E-003, US-007, US-002.EC-3. Paste `Check @README.md` into the composer without using the picker: it
stays plain text in the editor, and after send the transcript renders a `README.md` chip from daemon
metadata. From a terminal send to session B: ``Review @README.md and @session:<C>. see @README.md.
`@Component` @scope/pkg``. The delivered prompt has one README.md link (trailing `.` trimmed, duplicate
collapsed), the references block for C, and no mention for the code span or the missing `@scope/pkg`.
`compozy session history <B> --json` shows `mentions` with relative targets and UTF-16 offsets.

Agent origin: give sessions A and B different working directories in the same workspace. From A's
agent, call compozy__session_prompt targeting B with `run /some-skill and read @README.md and
@session:<C>`. The skill is not invoked; B's delivered prompt links B's README.md (not A's) and
carries C's reference. Cap leg: a prompt with 51 distinct existing files and 17 sessions delivers 50
links and 16 references; the rest stay text and the delivery record reports `dropped_over_cap`.
