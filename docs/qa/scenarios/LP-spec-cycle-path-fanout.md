---
id: LP-spec-cycle-path-fanout
area: LP
title: Resume spec-cycle fan-out from referenced task files
persona: Bruno
journey: J-01
expected: import_tasks returns ordered path and body_ref descriptors without body, each implementer opens the referenced task file, and a daemon restart during fan-out resumes the same tasks without duplicating embedded content, lanes, or completed work.
entry_points: compozy loop run --name implement-tasks; ext__spec_cycle__import_tasks; Web loop run detail
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260831-181713-956331-lab/qa-artifacts/qa/spec-cycle-task-runs.json; /Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260831-181713-956331-lab/qa-artifacts/qa/spec-cycle-result-post-restart.json
last_report: docs/qa/reports/2026-08-31-loop-result-fix.md
overlaps: LP-implement-tasks-orchestrated-mode; ET-spec-cycle-skill-bundle
---

Run implement-tasks against a task set whose files are large enough that embedding all bodies would dominate the action result. Confirm import_tasks returns no body field, workers read the exact path, and fan-out continues after a daemon restart without re-importing stale content or duplicating a lane.

QA impact 2026-08-31: hard cut from embedded task bodies to path and body_ref descriptors.

QA 2026-08-31: a public Loop invoked `ext__spec_cycle__import_tasks` against an authored task set. The result contained the ordered absolute `path` and content-addressed `body_ref`, serialized no `body`, and returned the same 365-byte descriptor payload after daemon restart. The canonical bundled-loop and coordinator suites own implementer prompt usage, fan-out ordering, and resumed output hydration.
