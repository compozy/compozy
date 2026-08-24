---
id: LP-runtime-selection-overrides
area: LP
title: Select matrix and single-selector per-task runtimes
persona: Bruno
journey: J-01
expected: One mixed task batch proves that a type plus complexity matrix rule applies only to its matching task, a legacy single-selector rule still applies, and an exact-ID rule overrides both; dry-run and real resolution return the same per-task runtime fields.
entry_points: compozy loop run --runtime ... --dry-run -o json; compozy loop run --runtime ... -o json; POST /api/workspaces/:workspace_id/loops/:name/run over HTTP and UDS
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /home/francisross/dev/qa-labs/compozy-runtime-selection-overrides-20260824-220605-045727-lab/qa-artifacts/qa/runtime-selection-blocked-verify.md
last_report: .superpowers/sdd/2026-08-24-compozy-conjunctive-runtime-rules/task-4-report.md
overlaps: LP-003; TA-079
---

story: As a delivery operator I can choose the right runtime for each task without changing the loop definition.

QA impact 2026-08-24: the matcher changed, so the prior pass and its stale evidence were reset before verification. In one mixed task batch, use a `type + complexity` matrix rule, a legacy single-selector rule, and an exact-ID override. Confirm the matrix selects only the task matching both fields, the single selector still resolves its task, and the exact-ID override wins. Capture the resolved runtime fields from dry-run and from a real submitted run, then confirm the equivalent HTTP or UDS request resolves the same batch.

Verification 2026-08-24: `blocked-verify`. CLI, HTTP, and UDS dry-runs returned the same effective three-rule configuration. Real CLI run `looprun-a7c7dc7d16f7ea2e` durably emitted `runtime_applied` for the matrix task (`codex/gpt-5.6-luna/high`), but the live action did not settle; after cancellation the isolated daemon recorded `loop: transition conflict: Goal session cleanup ... payload changed`. A fresh supported-model mixed batch (`looprun-1bea18dfb47059ae`) did not advance beyond generation start during bounded polling. The exact lab evidence is the `evidence` path above; this is not a pass claim.

src: .compozy/tasks/compozy-migration/task_06.md
