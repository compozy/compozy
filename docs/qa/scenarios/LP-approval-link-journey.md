---
id: LP-approval-link-journey
area: LP
title: Route one durable approval decision to the waiting node
persona: Marina
journey: J-03
expected: A human approval link identifies exactly one active gate wait, a decision atomically resumes or halts that node, duplicate decisions are harmless, ambiguity fails visibly, and a decision arriving during escalation cancels every remaining step.
entry_points: approval link; `compozy loop approve`; HTTP/UDS approval route; Web approval card
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-016
---

acceptance-walk: Open the real approval link from the waiting run, drop and restore the browser network during one decision, and race a duplicate CLI or HTTP decision. Confirm the exact active wait resumes or halts once, ambiguous or stale links fail visibly, later escalation steps cancel, and the refreshed Web card agrees with structured state.
