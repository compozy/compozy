# BUG-20260730-memory-reset-wrong-status: Unsupported full Memory reset returned the wrong status

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** J-25 reset derived memory, step 3
- **Scenarios:** MS-009
- **Found:** 2026-07-30 · **Report:** docs/qa/reports/2026-07-28-untested-full.md

## Summary

Dora received HTTP 400 for a well-formed but unsupported full reset instead of the documented semantic rejection.

## Reproduction

- **Charter:** CH-memory-reset-contract · **Tour:** Garbage Tour
- **Environment:** isolated current-source daemon / HTTP / en-US

1. POST a confirmed reset with `derived_only=false`.
2. Inspect the response status and code.

**Expected:** HTTP 422 with `memory.rejected`.
**Actual:** The pre-fix handler returned HTTP 400.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-root-fixes-retest-20260730-072350-328928-lab/qa-artifacts/qa/evidence/root-fixes/public-replay.md`

## Fix

- **Root cause:** The handler classified the supported-domain rejection as malformed input.
- **Fix commit:** bd0617c
- **Regression test:** `internal/api/core/memory_workspace_test.go`

## Verification

- **Retested:** 2026-07-30, same persona/journey · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Result:** Preview and confirmed derived-only requests return 200; a confirmed full reset returns 422 with the stable rejection code.
