# BUG-20260729-extension-update-partial-error: Native extension update hid committed partial progress

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-distribution, update all managed extensions
- **Scenarios:** ET-019
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Origin:** Fresh isolated native-tool replay

## Summary

An `all=true` native extension update committed the first target and correctly stopped when the
second target failed validation, but the public error was reduced to `tool_backend_failed` with
`backend_unhealthy`. The caller could not discover which target failed, how many targets completed,
or the cleanup warning attached to the committed update.

## Reproduction

1. Install two marketplace-managed extensions at version `1.0.0`.
2. Publish a valid `1.1.0` archive for the first target and an identity-invalid `1.1.0` archive for
   the second target.
3. Invoke `compozy__extensions_update` with `all=true` and `allow_unverified=true`.

**Expected:** The typed error identifies the failed target and completed count and carries every
committed update, including cleanup warnings, in `partial_result`.
**Actual before the fix:** The first extension reached `1.1.0` and emitted `extension.updated`, but
the native response exposed only a generic backend failure.

## Evidence

- Live registry, native-tool, installed-state, cleanup-warning, and event assertions:
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/016-extensions-lifecycle`.
- The pre-fix event store contains `extension.updated` for `qa-update-a` with
  `extension_update_cleanup_failed`, while the pre-fix native response reported
  `backend_unhealthy` without a partial result.

## Fix

- **Root cause:** The extension service returned both its committed update payloads and a typed
  `MarketplaceUpdateBatchError`, but the native adapter discarded the payload slice on every error.
  Dispatch then normalized the unclassified domain error into a generic backend failure.
- **Correction:** Batch failures now retain committed payloads in a bounded native `partial_result`
  with `failed_target` and `completed_count`. Extension-domain validation errors are mapped before
  dispatch normalization.
- **Fix commit:** `351f3535`
- **Regression test:** `Should preserve committed all-update results in a typed partial failure` in
  `internal/daemon/native_extension_tools_test.go`.

## Verification

- The focused native extension suite passes under the race detector.
- **Retested:** pending under Bruno; the 2026-07-29 Ada distribution charter supplied adjacent batch-update evidence.
