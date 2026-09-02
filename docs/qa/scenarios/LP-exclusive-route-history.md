---
id: LP-exclusive-route-history
area: LP
title: Author an exclusive route and inspect its durable cause
persona: Bruno
journey: J-complete-partial-loop
expected: A route node takes the first matching direct forward edge or its mandatory default, skips every dominated non-selected path without failure, omits inactive-route Tasks so they cannot block the selected downstream work, and exposes the exact decision cause consistently through CLI, HTTP, native-tool status, and SSE replay; a gate object route follows the same durable history contract.
entry_points: compozy loop validate|run|status; GET /loop-runs/:id over HTTP and UDS; compozy__loop_status; Loop SSE; web Loop editor and run detail; /docs/loops/dsl-reference; skills/compozy/references/loops.md
qa_status: pass
bug_ids: BUG-20260901-unselected-route-gate-executes
fix_status: fixed
retest_status: pass
fix_commits: 304059507bbeff0213b1d516cccbd5be7939bb03
evidence: docs/qa/reports/2026-09-01-loop-route-selection.md; docs/qa/bugs/BUG-20260901-unselected-route-gate-executes.md
last_report: docs/qa/reports/2026-09-01-loop-route-selection.md
overlaps:
---

acceptance-walk: Publish a Loop with two ordered route conditions, a default, a downstream join, and a gate `on_result` object route. Run matching and default inputs. Confirm exactly one route runs each time, dominated nodes read as `route_not_taken`, inactive-route Tasks are absent from the Task catalog and do not block the selected downstream work, broken CEL fails instead of defaulting, and every structured read shows the same `route_causes` facts.

2026-09-01: `pass` after fix — the focused planner regression and daemon integration journey confirmed that the unselected gate stayed `route_not_taken` with no verdict, while HTTP start and CLI/UDS read-back showed only the selected gate/action path and the run settled `done`.
