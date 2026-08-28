# BUG-20260827-live-uncurated-model-admission: Available live models fail session admission when not curated

- **Status:** fixed, pending remediation commit
- **Impact (user-side):** Blocks-Completion
- **Severity:** Major · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-20 Use a newly discovered model without a Compozy update
- **Scenarios:** MS-live-model-release-refresh; RT-cursor-logical-runtime-options
- **Found:** 2026-08-27 · **Report:** docs/qa/reports/2026-08-27-acp-runtime-catalog.md

## Summary

Runtime admission looked only at the curated catalog view. A model discovered live and available in the complete view could appear automatically yet still be rejected when a session tried to use it.

## Fix

- **Root cause:** explicit runtime validation called the default curated catalog projection instead of the complete availability projection.
- **Fix:** validate explicit models against `CatalogViewAll`; curation continues to control default browsing, not provider availability.
- **Fix commit:** pending; included in the single remediation commit
- **Regression test:** `TestCreateAppliesRuntimeModelOverride/Should start an advertised Cursor agent default exactly` asserts admission requests `CatalogViewAll` in the canonical Manager runtime suite.

## Verification

- **Focused automated result:** the Manager test passes with `-race` and proves an available live, non-curated model reaches runtime start.
- **Real provider canary:** a fresh live Cursor Grok 4.5 session completed its first prompt after the same admission path.
