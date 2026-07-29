---
id: ET-web-session-goal-strip
area: ET
title: Goal strip above the transcript with window-head lifecycle actions
persona: Bruno
journey:
expected: The newest session-origin Goal renders as one quiet line pinned above the transcript scroller — state dot (accent pulse active · warning blocked/paused · success done · faint moved) + GOAL kicker + objective + "turn a/b · ctx n%" mono facts — expanding to key/value rows (Contract, Run link, Context tokens + nudge threshold, Last verdict, Node/cause mono, Session link for moved goals, Draft goal command / Draft replacement staging /goal into the composer without sending). Pending/unknown context reads "ctx —" with an honest body sentence, never an invented percentage. Exactly one state-gated goal action rides the window head: Approve (needs-approval) > Resume (paused) > Pause (active) > Clear (terminal snapshot only). Goal prompts in the transcript render as one marker line; a failed goal read renders a session-level banner, not a full-bleed tinted bar.
entry_points: web session window head + goal zone; GET session goal; POST session prompt (/goal commands)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: GL-001, GL-005, GL-006, GL-008
---

story: As a person running goal work I read the goal's state at a glance in one line, reach the single relevant lifecycle action in the window head, and stage goal commands without anything auto-sending.

errors:

inventory: Needs QA — introduced by the session transcript redesign (goal strip + head actions, 2026-07-29).
