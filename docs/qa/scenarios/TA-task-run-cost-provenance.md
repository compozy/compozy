---
id: TA-task-run-cost-provenance
area: TA
title: Task run summary preserves truthful cost provenance
persona: Bruno
journey: J-24
expected: Task run detail and `agh task run show` agree on actual, exact five-bucket estimated, included, or unknown aggregate cost provenance; a missing active-bucket rate, included/unknown states, or incompatible child-session provenance fails closed without an amount.
entry_points: web task-run detail; `agh task run show <run-id> -o json`; HTTP and UDS task-run detail
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Inspect runs backed by homogeneous session provenance and a run whose sessions carry incompatible
cost states or sources. Reload the detail and compare Web, CLI, HTTP, and UDS projections. The
aggregate may expose a numeric amount only for compatible `actual/agent_reported` or estimated
`catalog_config`, `models_dev`, or `builtin` data; `included/none` and fail-closed `unknown/none`
results remain amountless.

Estimated session rows must price input, output, cache-read, cache-write, and reasoning independently;
remove one rate for a nonzero bucket and require the task aggregate to remain amountless and unknown.

Phase C planning 2026-07-19: companion to US-007 (task roll-up half). Forensic contract (SD-006):
timestamped Web/CLI/HTTP/UDS task-run detail captures for one homogeneous-provenance run and one
incompatible-provenance run failing closed to `unknown/none` with no amount.
