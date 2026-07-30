# BUG-20260729-provider-model-validation-status: Invalid model pricing returned HTTP 500

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-20, reject invalid provider model pricing
- **Scenarios:** MS-056; MS-provider-settings-model-delta-roundtrip
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Origin:** Fresh isolated Provider Settings validation replay

## Summary

Provider Settings correctly rejected a negative model rate without changing `config.toml`, but the
HTTP surface classified the caller validation error as an internal failure and returned 500.

## Reproduction

1. Read the effective Codex provider through Settings.
2. Set one curated model's `cost_input_per_million` to `-1` and PUT the complete provider payload.
3. Compare the response status and the config SHA-256 before and after the request.

**Expected:** HTTP 400 with the finite/non-negative validation message and no config mutation.
**Actual before the fix:** HTTP 500 with a generic internal-error envelope; config remained
unchanged.

## Evidence

- Rebuilt response, transport status, and before/after config hash:
  `/Users/pedronauck/dev/qa-labs/compozy-j20-model-lifecycle-20260729-144220-491767-lab/qa-artifacts/qa/evidence/045-model-catalog-lifecycle/negative-pricing-http.json`.

## Fix

- **Root cause:** `classifyProviderWrite` propagated `Config.Validate` errors without the Settings
  `ErrValidation` sentinel. The API mapper therefore treated a caller-owned config error as an
  unclassified server failure.
- **Correction:** Provider mutation validation now wraps the validation failure with the canonical
  Settings validation sentinel before transport mapping.
- **Fix commit:** pending completion gate
- **Regression test:** `Should reject negative provider model pricing without mutating config` in
  `internal/settings/config_apply_service_test.go`.

## Verification

- The rebuilt daemon returned HTTP 400 with the exact finite/non-negative field diagnostic.
- The before and after `config.toml` SHA-256 values were byte-identical.
- The full `internal/settings` race suite passes with 215 tests.
- **Retested:** rebuilt candidate green; governed fix commit pending
