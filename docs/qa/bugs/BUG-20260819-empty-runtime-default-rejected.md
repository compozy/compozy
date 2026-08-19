# BUG-20260819-empty-runtime-default-rejected: Empty runtime default cannot be saved

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Ada
- **Journey Step:** J-02 dry-run preview, configure reusable inputs
- **Scenarios:** LP-loop-input-defaults; LP-runtime-validation-preflight
- **Found:** 2026-08-19 · **Report:** docs/qa/reports/2026-08-19-typed-loop-inputs-remediation.md

## Summary

Ada could not save `{}` as a runtime input default even though runtime inputs allow any partial
combination of provider, model, and reasoning, including no selected fields.

## Reproduction

- **Charter:** CH-compozy-runtime-input-preflight · **Tour:** Garbage Tour
- **Environment:** desktop / wifi-fast / en-US, fresh isolated CLI and daemon

1. Publish a Loop with a required `runtime` input.
2. Run `compozy config set loops.inputs.<loop>.worker_runtime '{}' --scope workspace`.

**Expected:** The empty partial runtime is saved and returned as a present empty object.
**Actual:** The command fails with `table replacement requires at least one key`.

## Evidence

- Isolated public CLI replay recorded in the report and lab journey log.
- The failed write is independently absent from `compozy config get`.

## Fix

- **Root cause:** The config editor rejected empty TOML tables before the Loop default endpoint could
  persist the contract-valid empty runtime object.
- **Fix commit:** `46dd8ae`
- **Regression test:** `internal/config/persistence_test.go` and
  `internal/daemon/loop_api_runs_test.go`

## Verification

The public config read returned `{}` and a fresh CLI dry-run resolved the value from workspace
scope while preserving the global `slug` origin. The API defaults endpoint returned the same empty
object.
