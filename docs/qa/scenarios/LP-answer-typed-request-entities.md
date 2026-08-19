---
id: LP-answer-typed-request-entities
area: LP
title: Answer a human request with exact annotated entities
persona: Bruno
journey: J-supervise-loop-request
expected: Nested x-compozy-kind fields use the shared exact-entity controls, enum takes precedence, and a missing entity returns input_validation without closing or resuming the request
entry_points: web /loop-runs/:runId Needs-you card; CLI compozy loop respond; HTTP/UDS Loop respond route; compozy__loop_respond
qa_status: untested
bug_ids: BUG-20260818-nested-entity-picker-missing
fix_status: fixed
retest_status: pass
fix_commits: f3b8837
evidence: /Users/pedronauck/dev/qa-labs/compozy-typed-loop-inputs-20260819-015537-040869-lab/qa-artifacts/qa/typed-request-agent-picker.png; /Users/pedronauck/dev/qa-labs/compozy-typed-loop-inputs-20260819-015537-040869-lab/qa-artifacts/qa/journey-log.jsonl
last_report: docs/qa/reports/2026-08-18-typed-loop-inputs.md
overlaps: LP-ask-answer; LP-web-request-answer-card
---

Walk nested object and array annotations for the closed entity vocabulary. Verify the response error
reports `origin: response` and the exact nested field path, while ordinary JSON Schema failures keep
the `request_validation_failed` contract.
