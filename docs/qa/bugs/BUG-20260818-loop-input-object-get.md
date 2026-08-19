# BUG-20260818-loop-input-object-get: Operator cannot read a saved runtime input object

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Ada
- **Journey Step:** J-02 dry-run preview, inspect the reusable workspace default
- **Scenarios:** LP-loop-input-defaults
- **Found:** 2026-08-18 · **Report:** docs/qa/reports/2026-08-18-typed-loop-inputs.md

## Summary

Ada saved a partial runtime object through `compozy config set`, and both `config show` and the
Loop dry-run used it, but `compozy config get` said the exact saved path did not exist. Reading each
leaf separately worked, which forced Ada to reconstruct the object manually.

## Reproduction

- **Charter:** CH-compozy-runtime-input-preflight · **Tour:** Garbage Tour
- **Environment:** isolated macOS lab / CLI + UDS / en-US

1. Set `loops.inputs.typed-entity-qa.worker_runtime` to a provider/model/reasoning JSON object at
   workspace scope.
2. Restart the isolated daemon and confirm `config show` contains the object.
3. Run `compozy config get loops.inputs.typed-entity-qa.worker_runtime --workspace typed-loop-inputs`.

**Expected:** The exact object path returns the saved redacted-safe object.
**Actual:** The CLI reports that the path was not found, while leaf paths and effective dry-run
resolution succeed.

## Evidence

- `docs/qa/reports/2026-08-18-typed-loop-inputs.md`
- Independent read: the same lab's dry-run resolved the object with `origin: workspace`.

## Fix

- **Root cause:** `config get` searched only the flattened leaf projection; parent map paths were
  never resolved from the redacted config tree.
- **Fix commit:** `f3b8837`
- **Regression test:** `internal/cli/config_test.go` —
  `TestConfigCommandsManageDynamicLoopInputDefaults`.

## Verification

- The rebuilt candidate returns the complete provider/model/reasoning object from the exact parent
  path, and the same value resolves with `origin: workspace` in dry-run.
