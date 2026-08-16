# BUG-20260816-agent-plugin-validation-exit: Fatal portable validation exits successfully

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-agent-authoring, validate a fatal package
- **Scenarios:** ET-agent-plugin-validation
- **Found:** 2026-08-16 · **Report:** docs/qa/reports/2026-08-16-agent-plugins.md

## Summary

`compozy extension validate` printed a fatal portable-manifest error but exited with status 0. Scripts
could therefore treat an invalid package as releasable.

## Reproduction

- **Charter:** CH-agent-plugin-conformance · **Tour:** Garbage Tour
- **Environment:** macOS arm64, fresh isolated lab

1. Run `compozy extension validate <fatal-portable-fixture> -o json`.
2. Inspect both the structured error and process exit status.

**Expected:** The deterministic fatal code is returned with a nonzero process status.
**Actual:** The error payload was printed but the process returned status 0.

## Fix

- **Root cause:** The CLI renderer treated every successful validation command dispatch as a zero exit even when the returned portable validation result was fatal.
- **Fix commit:** pending the task 08 remediation checkpoint
- **Regression suite:** `internal/cli/root_test.go`

## Verification

- **Retested:** 2026-08-16 with the same fatal fixture against the rebuilt QA binary.
- **Result:** Pass — fatal validation returned status 1 while warning-only component skips remained status 0.
- **Evidence:** `docs/qa/evidence/2026-08-16-agent-plugins/conformance-checklist.json`; `internal/cli/root_test.go`.
