---
id: LP-web-timeline-graph-rows
area: LP
title: Read request, route, prune, amend and fork rows on the run timeline
persona: Bruno
journey: J-supervise-loop-request
expected: The story timeline renders a row per graph-completion beat: request opened/answered/expired/canceled with actor and decision, route taken naming the matched condition or stating the default was used, a pruned aggregate naming the lane count and cause, an amended row citing provenance, and a forked row linking the new run.
entry_points: /loop-runs/$runId story timeline
qa_status: untested
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: ""
last_report: docs/qa/reports/2026-08-18-graph-eng.md
overlaps: ""
---

story: As a Loop operator, I can read back exactly what happened to a run's requests, routes, and lanes without opening the raw event log.

src: .compozy/tasks/graph-eng/task_08.md
