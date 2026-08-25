---
id: NB-agent-call-publish
area: NB
title: Publish completed call evidence to a Network thread
persona: Bruno
journey: J-agent-call-publish
expected: A completed result publishes bounded evidence once per channel thread with source attribution; replay is idempotent and no reverse call mutation exists.
entry_points: compozy call publish; POST calls publish; compozy__call_publish; Network timeline
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-agent-call-golden-path
---

Publish one completed call through CLI and HTTP, replay it, and inspect the target Network timeline.
