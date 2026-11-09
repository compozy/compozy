---
id: TA-task-run-cost-provenance
area: TA
title: Task run summary preserves truthful cost provenance
persona: Bruno
journey: J-24
expected: Task run detail and `compozy task run show` agree on actual, exact five-bucket estimated, included, or unknown aggregate cost provenance; a missing active-bucket rate, included/unknown states, or incompatible child-session provenance fails closed without an amount.
entry_points: web task-run detail; `compozy task run show <run-id> -o json`; HTTP and UDS task-run detail
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-ta-replay-20260730-062156-531636-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
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

## Cumulative cost accounting (2026-11-09)

Run three turns reporting cumulative USD amounts 1, 3 and 4. Confirm per-turn costs 1, 2 and 1
in usage/turns, transcript usage parts and the live prompt stream, with session/task/daily total 4.
Release and resume the runtime, then restart the daemon: a restored report 5 contributes 1;
a reset report 0.5 contributes 0.5. An explicitly per-turn adapter keeps its reported amounts.
Upgrade a prior database with complete and missing session ledgers. Recomputed rows become actual;
missing/partial histories become unknown with no amount. Token counts, turn counts and timestamps
remain byte/value equivalent. Reopening must not repeat the repair.

Focused SQLite migration and prompt-contract tests are recorded in the harness efficiency report.
A fresh browser/HTTP/UDS scenario walk is not claimed by those tests.
