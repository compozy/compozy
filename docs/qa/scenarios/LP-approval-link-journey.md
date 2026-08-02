---
id: LP-approval-link-journey
area: LP
title: Route one durable approval decision to the waiting node
persona: Marina
journey: J-03
expected: A human approval link identifies exactly one active gate wait, a decision atomically resumes or halts that node, duplicate decisions are harmless, ambiguity fails visibly, and a decision arriving during escalation cancels every remaining step.
entry_points: approval link; `compozy loop approve`; HTTP/UDS approval route; Web approval card
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-016
---

QA impact 2026-08-02: Task 06 implements the durable gate wait, exact active-wait claim,
expiry ladder, decision race handling, and coordinator reservation. A real-user walk is blocked
until Task 07 ships the approval route and structured verbs; Task 08 owns the Web card and Task 13
owns the end-to-end approval-link walk.
