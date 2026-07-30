---
id: LP-loop-input-defaults
area: LP
title: Resolve workspace Loop input defaults with truthful origin
persona: Bruno
journey: J-02
expected: Global and workspace `[loops.inputs.<loop-name>]` values resolve per key as run, workspace, global, then definition; explicit false, zero, and valid empty values remain present; CLI, HTTP, UDS, and native config surfaces can inspect and mutate the intended scope; dry-run reports identical values/origins; unknown keys or type mismatches return typed `input_default` diagnostics and create no run or ACP session.
entry_points: config.toml; compozy config get/set/unset loops.inputs.<loop-name>.<key>; compozy loop run --dry-run -o json; GET/PUT/DELETE /api/workspaces/:workspace_id/loops/:name/input-defaults over HTTP and UDS; native config tools
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/loop-parity/cli-set-global.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/loop-parity/http-put-global.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/loop-parity/http-put-workspace.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/loop-parity/uds-get-defaults.json
last_report: docs/qa/reports/2026-07-27-devtool-oss-launch.md
overlaps: LP-runtime-selection-overrides; LP-runtime-validation-preflight
---

story: As a workspace operator I can configure reusable Loop inputs without hiding where each effective value came from.

QA impact 2026-07-27: added by Compozy migration Task 09. Flag only; the next QA cycle owns cross-surface, zero-value, isolation, and failure-path validation.

src: .compozy/tasks/compozy-migration/task_09.md
