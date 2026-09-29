# BUG-20260928-claude-inspection-close-timeout: A Claude session intermittently refuses its model after a slow session/close

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion (intermittent)
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Rafa, Théo
- **Journey Step:** J-15, first prompt of a Claude session (continued/forked child) after a model-catalog refresh
- **Scenarios:** RT-session-derive-native-fork; RT-session-derive-retry (observed); pre-existing on main
- **Found:** 2026-09-28 · **Report:** docs/qa/reports/2026-09-28-session-continue-fork-exec-b1.md

## Summary

The first prompt of a Claude child (`model: claude-haiku-4-5-20251001`) failed with `Provider configuration is unavailable:
acp: model "claude-haiku-4-5-20251001" is unavailable in config option "model"` (twice in the walk); an immediate retry worked.

## Root cause

The model-catalog refresh inspects claude-agent-acp with a short-lived session; `closeInspectedSession`
(`internal/acp/client_control.go`) gives `session/close` a 1 s budget and claude-agent-acp answers later. The expired budget
(`{"code":-32603,…"context deadline exceeded"}`) was reported as a failed inspection, the refresh dropped the model→transport
mapping, and the bind asked the adapter for the raw id. Daemon log: `daemon.model_catalog.refresh_failed … close inspected session`.

## Fix

An expired close budget is no longer an inspection failure (the options were already read; the process stop that follows owns
cleanup). Regression: `TestInspectSessionConfigOptionsDoesNotMutateTheACPNewSession/Should_keep_the_inspected_options_when_session/close_outlasts_its_budget`
(`internal/acp/client_start_contract_test.go`, helper scenario `config_options_slow_close`) — reproduces the exact error before the fix.

## Verification

After the fix every later Claude bind in the lab (continue child, retry walk) resolved the model on the first attempt; no further
`refresh_failed … close inspected session` lines.
