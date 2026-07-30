# BUG-20260730-loop-config-unknown-fields: Loop configuration silently accepted unknown fields

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-05 configure a Loop, step 4
- **Scenarios:** LP-017
- **Found:** 2026-07-30 · **Report:** docs/qa/reports/2026-07-28-untested-full.md

## Summary

Bruno could save a misspelled Loop configuration field and receive no error even though the runtime ignored it.

## Reproduction

- **Charter:** CH-loop-config-public-contract · **Tour:** Garbage Tour
- **Environment:** isolated current-source daemon / CLI / en-US

1. Configure `review-and-fix` with `bogus=true`.
2. Observe the command result.

**Expected:** The write fails and names the unknown field.
**Actual:** The pre-fix decoder accepted the field.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-root-fixes-retest-20260730-072350-328928-lab/qa-artifacts/qa/evidence/root-fixes/public-replay.md`

## Fix

- **Root cause:** CLI JSON decoding did not disallow unknown fields before constructing the Loop config request.
- **Fix commit:** bd0617c
- **Regression test:** `internal/cli/loop_test.go`

## Verification

- **Retested:** 2026-07-30, same persona/journey · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Result:** The public CLI exits 1 with `json: unknown field "bogus"`.
