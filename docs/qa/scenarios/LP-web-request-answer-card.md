---
id: LP-web-request-answer-card
area: LP
title: Answer a parked Loop request from the run page
persona: Bruno
journey: J-supervise-loop-request
expected: The Needs-you card renders the daemon's prompt, redacted context preview, and the persisted decision set only; an invalid answer shows field errors inline and leaves the request pending; a valid answer resolves the card from refreshed truth with no optimistic paint; a request answered elsewhere or on a terminated run shows the resolved outcome and no form.
entry_points: /loop-runs/$runId Needs-you card; parked panels; waits rail
qa_status: untested
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: ""
last_report: ""
overlaps: ""
---

story: As a Loop operator, I can answer an ask or decide a review from the run page and trust that every control I see is one the daemon actually authorized.

src: .compozy/tasks/graph-eng/task_08.md
