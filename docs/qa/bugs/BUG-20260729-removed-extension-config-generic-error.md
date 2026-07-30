# BUG-20260729-removed-extension-config-generic-error: CLI hides extension hard-cut replacement guidance

- **Status:** fixed
- **Impact (user-side):** Recovery friction
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Vera
- **Journey Step:** J-extension-policy-admin, reject a removed config path
- **Scenarios:** ET-extension-legacy-key-rejection
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-29-ext-improvs.md
- **Origin:** Task 11 isolated CLI policy replay

## Summary

`compozy config set extensions.marketplace.enabled false` rejected the removed path before mutation, but returned only `is not supported by config set`. The daemon config loader already names the hard-cut replacement family, so CLI users received weaker remediation than file-based config users.

## Reproduction

1. Run `compozy config set extensions.marketplace.enabled false -o json` against an isolated home.
2. Observe the command error and verify the config remains unchanged.

**Expected:** The error says the path was removed and directs the operator to `extensions.trust` or `extensions.sources`.
**Actual:** The error classified the path as merely unsupported and omitted the replacement.

## Fix

- **Root cause:** CLI path classification recognized only three historical leaves instead of the removed `extensions.marketplace` subtree owned by the config loader.
- **Correction:** Classify the entire removed subtree, preserving leaf-specific replacements where known and using the authoritative replacement families for every other leaf.
- **Fix commit:** pending Task 11 checkpoint
- **Regression test:** `TestConfigRenderingAndMutationHelpers/Should_reject_removed_extension_paths_with_replacements`.

## Verification

- Focused race regression: PASS.
- Fresh release binary returns `config path "extensions.marketplace.enabled" was removed; use "extensions.trust or extensions.sources"` before mutation.
