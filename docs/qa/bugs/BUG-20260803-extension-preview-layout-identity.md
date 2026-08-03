# BUG-20260803-extension-preview-layout-identity: Unchanged layouts appeared as removal and addition

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-kit-lifecycle, preview an already enabled kit
- **Scenarios:** ET-ext-preview
- **Found:** 2026-08-03 · **Report:** docs/qa/reports/2026-08-03-bundles-removal-review.md
- **Origin:** Deep-review remediation real-user QA

## Summary

After an extension kit was enabled without any content change, `compozy extension preview`
reported its window layout as both removed and added. The preview therefore contradicted the live
inventory and could lead an operator to expect a destructive replacement.

## Reproduction

1. Install and enable an extension that ships one window layout.
2. Run `compozy extension preview <name> -o json` without changing the package.
3. Observe one `removed` and one `added` row with the same resource ID.

**Expected:** An unchanged materialized layout is omitted from the canonical change set.
**Actual:** The authored name and materialized ID were treated as different resource identities.

## Evidence

- Before fix: `/Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/preview-enabled.json`.
- Passing re-walk: `/Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/preview-enabled-retest.json` and
  `/Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/preview-http-enabled-retest.json`.

## Fix

- **Root cause:** Layout publication materializes the authored layout name into a full resource ID,
  but preview decoded that full ID as the live resource name and compared it with the short
  authored name.
- **Correction:** Extension preview derives window-layout names from the canonical resource ID,
  matching the identity used by desired kit projection and inventory.
- **Fix commit:** pending Phase D checkpoint
- **Regression test:** `TestExtensionInventoryAndEnablePreview/Should omit an unchanged
  materialized layout`.

## Verification

- The focused regression passed under `-race`.
- The same enabled package returned `"changes":[]` through both CLI/UDS and HTTP after daemon
  restart.
