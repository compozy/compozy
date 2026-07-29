# BUG-20260729-marketplace-json-parity: Marketplace CLI JSON diverged from the daemon contract

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-agent-marketplace-parity, compare Marketplace search and detail across structured planes
- **Scenarios:** ET-cli-marketplace-search; ET-cli-marketplace-info
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Origin:** Fresh isolated CLI/HTTP/UDS parity replay

## Summary

`compozy marketplace search -o json` and `compozy marketplace info -o json` added a CLI-only
`resolution_source` field to otherwise shared daemon payloads. HTTP and UDS remained byte-identical,
so an agent could not treat the three structured planes as the same contract.

## Reproduction

1. Search one Marketplace kind through CLI JSON, HTTP, and UDS against the same daemon state.
2. Resolve the same stable entry identity through all three planes.
3. Compare the complete top-level payloads instead of decoding only known fields.

**Expected:** CLI JSON preserves the daemon-authored payload without transport-only fields.
**Actual before the fix:** Both CLI responses included `resolution_source`; HTTP and UDS did not.

## Evidence

- Exact pre-fix and rebuilt-candidate parity assertions:
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/022-marketplace-namespace`.

## Fix

- **Root cause:** Marketplace commands reused the generic workspace-aware JSON writer, which augments
  responses with resolution metadata. Typed tests decoded known fields and therefore ignored the
  unexpected top-level field.
- **Correction:** Marketplace search and detail route JSON output through the shared
  contract-preserving writer; their canonical CLI suite now inspects raw top-level fields.
- **Fix commit:** pending completion gate
- **Regression test:** Marketplace search and detail JSON parity cases in
  `internal/cli/marketplace_test.go`.

## Verification

- Focused Marketplace CLI race tests and the complete `internal/cli` race suite pass.
- The rebuilt CLI matches HTTP and UDS exactly for search, pagination, and detail.
- **Retested:** rebuilt candidate green; governed fix commit pending
