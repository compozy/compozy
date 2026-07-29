# BUG-20260729-provider-model-pricing-roundtrip: Provider Settings discarded model pricing changes

- **Status:** open
- **Impact (user-side):** Data-Loss
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-20, update provider model pricing through Settings
- **Scenarios:** MS-056; MS-provider-settings-model-delta-roundtrip
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Origin:** Fresh isolated HTTP/UDS/config lifecycle replay

## Summary

A Provider Settings PUT that kept the same curated model membership reported an applied change but
discarded explicit input, output, cache-read, cache-write, and reasoning rates. Reasoning settings
persisted, while the five pricing deltas did not reach `config.toml` or the catalog readback.

## Reproduction

1. Read the effective Codex provider through `GET /api/settings/providers/codex`.
2. Keep the same curated model IDs and set five distinct prices on `gpt-5.6-luna`.
3. PUT the provider through Settings, then read Settings, `compozy config show`, and the all-view
   model catalog.

**Expected:** All five explicit rates persist and round-trip without copying unchanged catalog
enrichment into `config.toml`.
**Actual before the fix:** The request was accepted, but the five rate fields stayed at their prior
effective values or remained absent from raw config.

## Evidence

- Runtime, transport, config, restart, and negative-branch assertions:
  `/Users/pedronauck/dev/qa-labs/compozy-j20-model-lifecycle-20260729-144220-491767-lab/qa-artifacts/qa/evidence/045-model-catalog-lifecycle`.

## Fix

- **Root cause:** Provider reconciliation compared only the desired and current curated membership
  sets. Equal sets returned the raw rows before comparing model fields, so pricing deltas were
  silently discarded.
- **Correction:** Reconciliation now projects the effective pre-write catalog state, compares every
  configurable model field, and persists only explicit deltas. Unchanged catalog labels, limits,
  and release metadata are not materialized into operator config.
- **Fix commit:** pending completion gate
- **Regression test:** `Should persist explicit five-rate changes without materializing unchanged
  catalog metadata` in `internal/settings/config_apply_service_test.go`.

## Verification

- The repaired daemon persisted `1.1`, `2.2`, `0.11`, `0.22`, and `3.3` for the five buckets.
- Settings, `config show`, HTTP/UDS catalog readback, and a daemon restart retained the exact values.
- The full `internal/settings` race suite passes with 215 tests.
- **Retested:** rebuilt candidate green; governed fix commit pending
