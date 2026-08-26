---
id: LP-orchestrate-tasks-delegation
area: LP
title: Deliver an authored spec through the retired orchestrate-tasks Loop
persona: Bruno
journey: J-01
expected: Historical only — the standalone orchestrate-tasks Loop is absent after its behavior moves into implement-tasks mode=orchestrated.
entry_points: compozy loop list; /marketplace/bundled/spec-cycle
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-goal-command-judge; ET-spec-cycle-skill-bundle
---

Retired — the standalone `orchestrate-tasks` catalog entry was hard-deleted. Its delegation contract
is now owned by `LP-implement-tasks-orchestrated-mode`, reached through `implement-tasks` with
`mode=orchestrated`. Historical reports keep this stable id for lookup.
