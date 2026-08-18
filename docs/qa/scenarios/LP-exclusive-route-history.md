---
id: LP-exclusive-route-history
area: LP
title: Author an exclusive route and inspect its durable cause
persona: Bruno
journey: J-complete-partial-loop
expected: A route node takes the first matching direct forward edge or its mandatory default, skips every dominated non-selected path without failure, and exposes the exact decision cause consistently through CLI, HTTP, native-tool status, and SSE replay; a gate object route follows the same durable history contract.
entry_points: compozy loop validate|run|status; GET /loop-runs/:id over HTTP and UDS; compozy__loop_status; Loop SSE; web Loop editor and run detail; /docs/loops/dsl-reference; skills/compozy/references/loops.md
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

acceptance-walk: Publish a Loop with two ordered route conditions, a default, and a gate `on_result` object route. Run matching and default inputs. Confirm exactly one route runs each time, dominated nodes read as `route_not_taken`, broken CEL fails instead of defaulting, and every structured read shows the same `route_causes` facts.
