---
title: Large Loop action results survive instead of breaking the run
type: feature
---

A Loop action can now return a payload larger than the task-run envelope without losing coordinator restart safety. Results up to 16 KiB stay inline; anything larger is stored in the existing Loop blob store and read back byte for byte through a workspace-authorized paging resource. A result above the action budget fails with the typed `action_result_too_large` error before the task completes, so the lease is released instead of the run stalling. (#510)

- Adds `compozy task run result`, task-run result paging over HTTP and UDS, the Host API `tasks/runs/result` resource, and the `compozy__task_run_result` native tool.
- `compozy__tool_list` now returns deterministic pages so the global tool-result limit stays enforceable; `compozy__tool_info` still returns full descriptors.
- The Task UI shows and copies bounded results.
- Spec-cycle Task fan-out is a hard cut from embedded bodies to `path` plus `body_ref`.

No `config.toml` key was added or removed — the existing tool-result budget remains the source of truth.
