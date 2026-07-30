# BUG-20260730-network-zero-counters-omitted: Network status hid zero-value counters

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Ada
- **Journey Step:** J-administer-network-live inspect runtime state, step 2
- **Scenarios:** NB-001
- **Found:** 2026-07-30 · **Report:** docs/qa/reports/2026-07-28-untested-full.md

## Summary

Ada received an incomplete Network status document whenever counters were zero, so absence was indistinguishable from an unsupported field.

## Reproduction

- **Charter:** CH-network-status-empty-runtime · **Tour:** Feature Tour
- **Environment:** isolated current-source daemon / CLI / en-US

1. Start a fresh daemon with Network ready and no traffic.
2. Read `compozy network status -o json`.

**Expected:** Every documented counter and collection is present with zero or an empty array.
**Actual:** The pre-fix contract omitted zero-value fields.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-root-fixes-retest-20260730-072350-328928-lab/qa-artifacts/qa/evidence/root-fixes/public-replay.md`

## Fix

- **Root cause:** Network response fields used omission semantics for required zero-value facts.
- **Fix commit:** bd0617c
- **Regression test:** `internal/api/core/network_test.go`

## Verification

- **Retested:** 2026-07-30, same persona/journey · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Result:** The CLI returns every zero counter plus empty `declared_channels` and `kind_metrics` arrays.
