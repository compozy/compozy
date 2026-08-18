---
id: LP-web-request-answer-card
area: LP
title: Answer a parked Loop request from the run page
persona: Bruno
journey: J-supervise-loop-request
expected: The Needs-you card renders the daemon's prompt, redacted context preview, and persisted decision set only; requests from different generations stay distinct; context failures expose retry for the exact request; invalid answers remain pending with inline errors; valid answers resolve from refreshed truth with no optimistic paint; and requests answered elsewhere or on terminated runs show the outcome without a form.
entry_points: /loop-runs/$runId Needs-you card; parked panels; waits rail
qa_status: pass
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: /Users/pedronauck/dev/qa-labs/compozy-graph-eng-review-20260818-141718-102629-lab/qa-artifacts/qa/screenshots/loop-request-repeated-generation.png
last_report: docs/qa/reports/2026-08-18-graph-eng-review.md
overlaps: ""
---

story: As a Loop operator, I can answer an ask or decide a review from the run page and trust that every control I see is one the daemon actually authorized.

src: .compozy/tasks/graph-eng/task_08.md
