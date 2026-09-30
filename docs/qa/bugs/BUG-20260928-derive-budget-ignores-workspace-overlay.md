# BUG-20260928-derive-budget-ignores-workspace-overlay: [session.derive] in a workspace overlay has no effect

- **Status:** fixed
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-14 derive branch: preview line in the Continue/Fork dialog
- **Scenarios:** ET-web-session-continue
- **Found:** 2026-09-28 · **Report:** docs/qa/reports/2026-09-28-session-continue-fork-exec-b2.md

## Summary

A workspace's `.compozy/config.toml` sets `[session.derive] max_replay_bytes = 4096` (spec §746: "workspace overlay may override"). Continuing a 10.8 KiB session from that workspace still carried everything, and the preview read "Carries over 12 messages · 10.8 KiB" with no truncation.

## Root cause

`Manager.deriveBudget()` (`internal/session/derive_prepare.go`) read only the manager's global `deriveConfig` and never the source workspace's resolved config.

## Fix

`deriveBudget(*workspacepkg.ResolvedWorkspace)` uses the resolved workspace's `Session.Derive` when it is set; the preview resolves the source workspace (`resolveResumeWorkspace`) and the commit passes the workspace it already resolved, so preview and derive share one budget (ADR-002). Regression: `TestDerivePreview/Should_bound_the_preview_and_the_continue_by_the_source_workspace_overlay` (`internal/session/manager_derive_test.go`). Re-walked: "Carries over 4 of 12 messages · 3.7 KiB" + "8 earlier messages omitted to fit the context budget." (VC-08).

## Note

The overlay needs `max_message_bytes` ≤ `max_replay_bytes` too (default 16384). Setting only `max_replay_bytes = 4096` fails validation, and `POST /api/workspaces/resolve` answers a bare 500 "Internal Server Error" (daemon log names the rule). Recorded as a paper cut in the report.
