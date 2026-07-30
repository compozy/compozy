---
id: TA-agent-knowledge-refresh-on-wake
area: TA
title: Refresh changed workspace knowledge on a later worker wake
persona: Priya
journey: consumer-saas-growth
expected: An active task-role worker observes changed workspace knowledge on its next eligible wake and acts on the new signal without a second operator prompt.
entry_points: workspace knowledge; task-role wake; hosted native task lease; Network channel
qa_status: fail
bug_ids: BUG-20260729-agent-knowledge-refresh-missed
fix_status: pending
retest_status: pending
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/probes/silent-event-drop.json
last_report: docs/qa/reports/2026-07-29-ext-improvs.md
overlaps: TA-task-role-session-activation
---

Task 11 changed the Data Scientist's event-volume knowledge from `first_save: 7812` to
`first_save: 0`. The session processed three later review wake turns but did not re-read or report
the new value within the five-minute recovery window. No follow-up operator prompt was sent.

This scenario owns knowledge freshness across turns. TA-task-role-session-activation continues to
own activation, native claim, and single-run execution.
