---
id: LP-web-timeline-graph-rows
area: LP
title: Read request, route, prune, amend and fork rows on the run timeline
persona: Bruno
journey: J-supervise-loop-request
expected: The story timeline renders a row per graph-completion beat: request opened/answered/expired/canceled with actor and decision, route taken naming the matched condition or stating the default was used, a pruned aggregate naming the lane count and cause, an amended row citing provenance, and a forked row linking the new run.
entry_points: /loop-runs/$runId story timeline
qa_status: pass
bug_ids: ""
fix_status:
retest_status: pass
fix_commits: ""
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-task07-final-web-20260822-131622-550786-lab/qa-artifacts/qa/task07-scenario-walks.md; .compozy/tasks/loop-task-legibility/evidence/visual/task_05/VC-17; .compozy/tasks/loop-task-legibility/evidence/visual/task_05/VC-23
last_report: docs/qa/reports/2026-08-21-loop-task-legibility.md
overlaps: ""
---

story: As a Loop operator, I can read back exactly what happened to a run's requests, routes, and lanes without opening the raw event log.

src: .compozy/tasks/graph-eng/task_08.md
