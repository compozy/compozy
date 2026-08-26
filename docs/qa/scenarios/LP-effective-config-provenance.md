---
id: LP-effective-config-provenance
area: LP
title: Explain which Loop config source won
persona: Ada
journey: J-configure-and-run-loop
expected: Inspect and dry-run report the current effective config with a JSON Pointer source for every field, while run status preserves the source map pinned at admission after current defaults change.
entry_points: compozy loop inspect -o json; compozy loop run --dry-run -o json; compozy loop status -o json; GET /api/workspaces/:workspace_id/loop-runs/:run_id; compozy__loop_inspect; compozy__loop_status
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-20260826-192350-462800-lab/qa-artifacts/qa/logs/inspect-builtin.json; /Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-20260826-192350-462800-lab/qa-artifacts/qa/logs/configure-stored.json; /Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-20260826-192350-462800-lab/qa-artifacts/qa/logs/inspect-stored.json; /Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-20260826-192350-462800-lab/qa-artifacts/qa/logs/halt-run-status-pinned.json; /Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-20260826-192350-462800-lab/qa-artifacts/qa/logs/halt-run-native.json; /Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-rerun-20260826-200713-569291-lab/qa-artifacts/qa/logs/dry-run-zero-overrides.json; /Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-rerun-20260826-200713-569291-lab/qa-artifacts/qa/logs/api-status-current-head.json
last_report: docs/qa/reports/2026-08-26-loop-issues-fixes.md
overlaps: LP-runtime-provenance-observation; LP-loop-config-file-snake-case
---

Walk explicit zero and false overrides, delivery and watch defaults, stored Loop config, and per-run
config. Confirm paths such as `/iteration_cap` and `/runtime_defaults/worker/model` name the logical
winning source and that changing current config cannot rewrite an existing run's status.

QA 2026-08-26: CLI inspect and configure covered built-in and stored sources, while dry-run
preserved explicit zero and false overrides with `per_run` sources. After current config changed,
the admitted run kept its pinned source map across CLI, HTTP, and the native status tool. The fresh
current-head rerun confirmed the same per-run provenance.
