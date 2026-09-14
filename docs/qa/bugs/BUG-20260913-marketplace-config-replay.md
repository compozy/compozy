# BUG-20260913-marketplace-config-replay: Unrelated config application removes newly registered sources

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-marketplace-acquisition, retain a source through extension installation
- **Scenarios:** ET-web-marketplace-sources-add
- **Found:** 2026-09-13 · **Report:** docs/qa/reports/2026-09-13-marketplace-catalog.md

## Reproduction

Register a plugin marketplace through the public endpoint and install its extension. A subsequent
unrelated runtime configuration apply can replay the earlier Marketplace snapshot, leaving Settings
with only the curated feed. E2E-005 reproduced the disappearance after installing loop-engineering.

## Root Cause and Repair

Source registration persists and reconfigures through marketplaceRuntime. The daemon Settings applier
unconditionally reconciled Marketplace on every apply and rollback, even when that operation changed
no Marketplace configuration. It could overwrite independently registered live sources with its older
snapshot. Apply and rollback now reconcile Marketplace only when that configuration actually changes,
matching the existing model-catalog reconciliation ownership. Explicit feed/source configuration
changes still apply and roll back through the same path.

Invariant: unrelated config apply and rollback preserve independently registered sources.
Owner/canonical suite: TestDaemonSettingsRuntimeApplier with real SQLite and source registration.
Both success and rollback cases reproduced the lost source before the fix. Full canonical Settings runtime applier race verification passed in3.862s.
A fresh E2E-005 browser walk passed, preserving source diagnostics, toggles, removal and re-add. No test assertion was relaxed.
