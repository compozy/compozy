# BUG-20260715-native-marketplace-extension-parity: Native Marketplace search drops extensions after daemon boot

- **Status:** verified
- **Impact (user-side):** Breaks agent parity
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-agent-marketplace-parity, unified discovery
- **Scenarios:** ET-049; ET-api-marketplace-namespace; ET-cli-marketplace-search
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-15-marketplace.md
- **Origin:** Task 11 isolated Marketplace QA

## Summary

For one unchanged daemon state, CLI, HTTP, and UDS returned the curated extension while registered native tool `agh__marketplace_search` returned an empty extension slice with `extension service is not configured`. Agents therefore saw a materially different catalog than human and structured transport clients.

## Reproduction

1. Start the daemon with a curated extension feed containing `qa-marketplace-extension`.
2. Refresh the catalog and confirm CLI, HTTP, and UDS Marketplace search each return the entry.
3. Invoke `agh__marketplace_search` with `{}` through the live tool registry.

**Expected:** Fixed kind order and per-kind items/errors match the shared daemon projection across CLI, HTTP, UDS, and native discovery.
**Actual:** Only the native extension kind failed with `extension service is not configured`.

## Evidence

- Field-level isolated replay: `/Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/native-marketplace-extension-parity.json`.
- The native descriptor remained registered, available, authorized, and callable, proving this was handler wiring rather than policy denial.

## Fix

- **Root cause:** `bootToolRegistry` runs before `bootExtensions`; `nativeToolsDeps` copied the then-nil extension service into the native adapter instead of resolving the service after boot.
- **Correction:** The dependency is now a late-bound accessor over `bootState`. Marketplace native discovery and native extension lifecycle calls resolve the current daemon service at call time.
- **Fix commit:** pending Phase D checkpoint
- **Regression test:** The canonical native Marketplace suite builds dependencies before attaching the extension service, attaches it afterward as the real boot does, and requires the extension entry to be projected.

## Verification

- The canonical regression failed before the production change with `extension service is not configured`.
- `TestMarketplaceNativeSearch` and the complete native extension tool suite pass under `-race`.
- After rebuilding and restarting the isolated daemon, `agh__marketplace_search` returned the exact four-kind order and `qa-marketplace-extension` with no extension error, matching CLI, HTTP, and UDS.
