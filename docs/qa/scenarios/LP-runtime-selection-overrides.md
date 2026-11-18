---
id: LP-runtime-selection-overrides
area: LP
title: Select matrix and single-selector per-task runtimes
persona: Bruno
journey: J-01
expected: CLI, HTTP, and UDS dry-runs expose the same ordered effective matrix, legacy single-selector, and exact-ID rule stack. A settled live mixed batch then proves the matrix applies only when both fields match, the legacy rule still applies, and the exact-ID rule overrides both through each item's resolved_runtime fields and provenance.
entry_points: compozy loop run --runtime ... --dry-run -o json; compozy loop run --runtime ... -o json; POST /api/workspaces/:workspace_id/loops/:name/run over HTTP and UDS
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /home/francisross/dev/qa-labs/compozy-runtime-selection-overrides-20260824-220605-045727-lab/qa-artifacts/qa/runtime-selection-blocked-verify.md
last_report: docs/qa/reports/2026-08-24-conjunctive-runtime-rules.md
overlaps: LP-003; TA-079
---

story: As a delivery operator I can choose the right runtime for each task without changing the loop definition.

QA impact 2026-08-24: the matcher changed, so the prior pass and its stale evidence were reset before verification. In one mixed task batch, use a `type + complexity` matrix rule, a legacy single-selector rule, and an exact-ID override. First confirm CLI, HTTP, and UDS dry-runs expose the same effective ordered rule stack. Then submit a live run, wait for the mixed batch to settle, and inspect every item's `resolved_runtime` fields and per-field provenance. Dry-run configuration parity is not per-item resolution evidence.

Verification 2026-08-24: `blocked-verify`. CLI, HTTP, and UDS dry-runs proved only parity of the effective ordered three-rule configuration. Real CLI run `looprun-a7c7dc7d16f7ea2e` durably emitted `runtime_applied` for the matrix task (`codex/gpt-5.6-luna/high`), but the live action did not settle, so the mixed batch's per-item `resolved_runtime` and provenance remain unproved. After cancellation the isolated daemon recorded `loop: transition conflict: Goal session cleanup ... payload changed`. A fresh supported-model mixed batch (`looprun-1bea18dfb47059ae`) did not advance beyond generation start during bounded polling. The exact lab evidence is the `evidence` path above; this is not a pass claim.

src: .compozy/tasks/compozy-migration/task_06.md

QA impact 2026-11-11 (#1517): after a mixed batch settles, restart the isolated runtime and compare
each item's applied ACP options and per-option provenance across HTTP, UDS, CLI, native status and
replayed `runtime_applied` SSE frames. Include the automatic Codex Full access mode; absent options
remain omitted. The storage contract also covers an explicitly false boolean option. Historical
append-only events are not rewritten by the serializer repair.

Focused verification 2026-11-11: the existing applied-runtime storage case fails before the repair
and passes afterward, including boolean false and per-option provenance. The existing mixed/matrix
runtime integration and cross-surface restart/replay case pass with SQLite and the isolated ACP
test peer. This verifies the projection leg; it does not change the historical live-provider
scenario verdict above.

QA impact (#1520, #1526): applied ACP provenance is rebuilt per applied option. In the
run-agent action checks, an agent-added mode gets `agent` alongside a node-authored
`fast_mode=false`; a skipped mode loses its source entry while surviving node provenance
is retained. Reuse the existing runtime integration for persisted and replayed projections;
these metadata repairs do not alter runtime selection or the historical live-provider verdict.

Focused verification 2026-11-11 (#1520, #1526): the three action cases fail on the original
implementation and pass after the repair. The unchanged runtime integration passes, including
persisted/read projections and replay after isolated daemon restart. Scoped Loop lint reports
zero issues for both normal and integration build tags; broad gates remain CI-owned.
