---
id: LP-runtime-validation-preflight
area: LP
title: Validate runtime selections at their owning boundaries
persona: Dora
journey: J-02
expected: Static and runtime-routing errors remain deterministic `runtime_validation` items, while invalid declared runtime inputs return field-addressed `input_validation`; unknown providers fail before ACP spawn, exact model IDs pass unchanged for known providers, and every preflight-rejected path materializes no run or session.
entry_points: compozy loop validate; compozy loop run --dry-run; POST /api/workspaces/:workspace_id/loops/:name/validate and /run over HTTP and UDS
qa_status: pass
bug_ids: BUG-20260819-empty-runtime-default-rejected
fix_status: fixed
retest_status: pass
fix_commits: 46dd8ae
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-runtime-graph-fixes-20260819-234724-890442-lab/qa-artifacts/qa/logs/dora-runtime-dry-run-fast.json; /Users/pedronauck/dev/qa-labs/compozy-loop-runtime-graph-fixes-20260819-234724-890442-lab/qa-artifacts/qa/logs/dora-http-runtime-fast.json; /Users/pedronauck/dev/qa-labs/compozy-loop-runtime-graph-fixes-20260819-234724-890442-lab/qa-artifacts/qa/logs/dora-final-runs.json
last_report: docs/qa/reports/2026-08-19-loop-runtime-graph-fixes.md
overlaps: LP-022; LP-runtime-selection-overrides
---

story: As an operator I receive actionable runtime diagnostics before any invalid provider process starts.

QA impact 2026-07-26: added by Compozy migration Task 06. Flag only; the next QA cycle owns adversarial preflight validation.

src: .compozy/tasks/compozy-migration/task_06.md

QA impact 2026-08-05: model catalog rows are metadata, not runtime membership. Reset for a public
dry-run walk proving unknown-provider rejection and exact-model passthrough without ACP spawn.

QA 2026-08-05: CLI/UDS and HTTP dry-runs preserved `cursor/composer-2.5`, rejected
`flarp/anything` as `unknown_provider`, and left the durable run count at zero.

QA impact 2026-08-05 (review remediation): the Cursor registration and exact-model effective-config
contract gained stronger integration coverage. Reset for the exact-ID public readback canary.

QA 2026-08-05 (review remediation): CLI, HTTP, and UDS dry-runs preserved
`worker=cursor/composer-2.5`; unknown provider still failed and no Loop run was created.

QA 2026-08-18: CLI, HTTP/UDS, native dry-run, and Web preserved an exact custom model and partial
runtime. Invalid provider/entity values returned field-addressed `input_validation` before a run.

QA impact 2026-08-19: runtime inputs can now bind directly to run-agent and Goal nodes. Reset to
verify exact-reference validation, input provenance, and rejection before ACP spawn.

QA impact 2026-08-19: runtime speed is closed to `normal|fast` in definitions, task frontmatter,
config, runtime inputs, and per-run flags. Verify invalid speed fails before ACP spawn.
