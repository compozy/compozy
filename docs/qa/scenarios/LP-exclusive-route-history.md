---
id: LP-exclusive-route-history
area: LP
title: Author an exclusive route and inspect its durable cause
persona: Bruno
journey: J-complete-partial-loop
expected: A route node takes the first matching direct forward edge or its mandatory default, skips every dominated non-selected path without failure, omits inactive-route Tasks so they cannot block the selected downstream work, and exposes the exact decision cause consistently through CLI, HTTP, native-tool status, and SSE replay; a gate object route follows the same durable history contract.
entry_points: compozy loop validate|run|status; GET /loop-runs/:id over HTTP and UDS; compozy__loop_status; Loop SSE; web Loop editor and run detail; /docs/loops/dsl-reference; skills/compozy/references/loops.md
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: passed
fix_commits: 30405950134c99f0feea2d97fb6acf81f357bc7e
evidence: go test -tags=integration ./internal/daemon -run TestDaemonE2ELoopGenerationFeedbackShouldConvergeAndBound/Should_route_classifier_output_and_expose_gate_reroute_causes_through_loop_status -count=1; /Users/pedronauck/dev/qa-labs/compozy-issue-479-exclusive-route-20260825-213535-019283-lab/qa-artifacts/qa/notes/cli-runtime-verdict.json; /Users/pedronauck/dev/qa-labs/compozy-issue-479-exclusive-route-20260825-213535-019283-lab/qa-artifacts/qa/notes/cli-task-catalog-verdict.json; /Users/pedronauck/dev/qa-labs/compozy-issue-479-exclusive-route-20260825-213535-019283-lab/qa-artifacts/qa/notes/http-independent-verdict.json
last_report: docs/qa/reports/2026-09-01-loop-route-selection.md
overlaps:
---

acceptance-walk: Publish a Loop with two ordered route conditions, a default, a downstream join, and a gate `on_result` object route. Run matching and default inputs. Confirm exactly one route runs each time, dominated nodes read as `route_not_taken`, inactive-route Tasks are absent from the Task catalog and do not block the selected downstream work, broken CEL fails instead of defaulting, and every structured read shows the same `route_causes` facts.

2026-09-01: `pass` — the exact fix head passed the CoordinatorRunner regression, focused race suite, `make gate`, and the public CLI/API route-history E2E. The unselected gate stayed `route_not_taken`; only the selected gate/action path ran and the run settled `done`.
