---
id: LP-runtime-validation-preflight
area: LP
title: Validate runtime selections at their owning boundaries
persona: Dora
journey: J-02
expected: Static loop validation reports definition-owned runtime errors, while dry-run and submission report effective workspace/run errors as deterministic `runtime_validation` items; unknown providers fail before ACP spawn, exact model IDs pass unchanged for known providers, provider model rejection stays at the ACP bind/prompt boundary, missing bound secrets fail before ACP spawn, and every preflight-rejected path materializes no session.
entry_points: compozy loop validate; compozy loop run --dry-run; POST /api/workspaces/:workspace_id/loops/:name/validate and /run over HTTP and UDS
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/loop-parity/invalid-input.stdout.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/loop-parity/invalid-runtime.http.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/loop-parity/runs-before-invalid.normalized.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/loop-parity/runs-after-invalid.normalized.json; /Users/pedronauck/dev/qa-labs/compozy-issue-312-cursor-models-20260805-200518-943803-lab/qa-artifacts/qa/issue-312-evidence.md;/Users/pedronauck/dev/qa-labs/compozy-issue-312-review-remediation-final-20260805-230015-520918-lab/qa-artifacts/qa/issue-312-review-evidence.md
last_report: docs/qa/reports/2026-08-05-issue-312-review-remediation.md
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
