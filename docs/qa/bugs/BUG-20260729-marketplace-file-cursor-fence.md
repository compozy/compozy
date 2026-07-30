# BUG-20260729-marketplace-file-cursor-fence: Identical file-catalog refresh invalidated pagination

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-agent-marketplace-parity, continue a single-kind Marketplace page
- **Scenarios:** ET-api-marketplace-namespace; ET-cli-marketplace-search
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Origin:** Fresh isolated three-entry file-catalog replay

## Summary

A cursor returned from the first page of an unchanged local file catalog was rejected on the next
request. The file source refreshes on access, and the cursor fence treated the resulting fetch time
as catalog identity even when catalog content and revision were unchanged.

## Reproduction

1. Configure a three-entry `file://` Marketplace catalog and refresh it through the public API.
2. Request an extension page with `limit=2` and retain its `next_cursor`.
3. Request the next page with that cursor without changing the catalog.

**Expected:** The second page returns the remaining unique entry and no continuation cursor.
**Actual before the fix:** The second request returned 400 with cursor-revision restart guidance.

## Evidence

- Pre-fix failure, stable continuation, cursor-binding checks, and real revision invalidation:
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/022-marketplace-namespace`.

## Fix

- **Root cause:** `marketplaceCatalogFence` hashed `FetchedAt`, so every identical file refetch
  changed the projection fence.
- **Correction:** The fence now uses the catalog's authored `GeneratedAt` revision and content
  projection, while freshness remains diagnostic metadata. A genuinely newer revision still rejects
  an old cursor.
- **Fix commit:** `351f3535`
- **Regression test:** The existing single-kind page-continuation subtest in
  `internal/api/core/marketplace_test.go` now covers unchanged refetch and changed revision.

## Verification

- Focused and complete `internal/api/core` race suites pass.
- Rebuilt CLI, HTTP, and UDS return the same three unique entries across two pages.
- Query, kind, scope, workspace, and authored-revision mismatches reject the cursor deterministically.
- **Retested:** 2026-07-29, rebuilt pagination replay green; fix shipped in `351f3535`
