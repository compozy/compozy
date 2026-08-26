---
id: LP-effective-config-provenance
area: LP
title: Explain which Loop config source won
persona: Ada
journey: J-configure-and-run-loop
expected: Inspect and dry-run report the current effective config with a JSON Pointer source for every field, while run status preserves the source map pinned at admission after current defaults change.
entry_points: compozy loop inspect -o json; compozy loop run --dry-run -o json; compozy loop status -o json; GET /api/workspaces/:workspace_id/loop-runs/:run_id; compozy__loop_inspect; compozy__loop_status
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-runtime-provenance-observation; LP-loop-config-file-snake-case
---

Walk explicit zero and false overrides, delivery and watch defaults, stored Loop config, and per-run
config. Confirm paths such as `/iteration_cap` and `/runtime_defaults/worker/model` name the logical
winning source and that changing current config cannot rewrite an existing run's status.
