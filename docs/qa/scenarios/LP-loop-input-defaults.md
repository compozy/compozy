---
id: LP-loop-input-defaults
area: LP
title: Resolve workspace Loop input defaults with truthful origin
persona: Bruno
journey: J-02
expected: Global and workspace `[loops.inputs.<loop-name>]` values resolve per key as run, workspace, global, then definition; scalar and partial runtime objects round-trip through CLI, HTTP, UDS, and native config surfaces; dry-run reports identical values/origins; invalid values return `input_validation` and create no run or ACP session.
entry_points: config.toml; compozy config get/set/unset loops.inputs.<loop-name>.<key>; compozy loop run --dry-run -o json; GET/PUT/DELETE /api/workspaces/:workspace_id/loops/:name/input-defaults over HTTP and UDS; native config tools
qa_status: pass
bug_ids: BUG-20260818-loop-input-object-get
fix_status: fixed
retest_status: pass
fix_commits: pending implementation commit
evidence: /Users/pedronauck/dev/qa-labs/compozy-typed-loop-inputs-20260819-015537-040869-lab/qa-artifacts/qa/journey-log.jsonl
last_report: docs/qa/reports/2026-08-18-typed-loop-inputs.md
overlaps: LP-runtime-selection-overrides; LP-runtime-validation-preflight
---

story: As a workspace operator I can configure reusable Loop inputs without hiding where each effective value came from.

QA impact 2026-07-27: added by Compozy migration Task 09. Flag only; the next QA cycle owns cross-surface, zero-value, isolation, and failure-path validation.

src: .compozy/tasks/compozy-migration/task_09.md

QA 2026-08-18: a partial runtime object round-tripped through exact `config get`, effective
workspace defaults, dry-run values, and origin reporting after the object-path reader fix.
