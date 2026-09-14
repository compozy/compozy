# BUG-20260912-context-delivery-details: Context delivery details disappear across session boundaries

- **Status:** fixed
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Rafa
- **Journey Step:** J-14 read a finished transcript, context provenance
- **Scenarios:** ET-cli-session-usage-context; RT-acp-usage-cache-meta
- **Found:** 2026-09-12 · **Report:** ../reports/2026-09-12-session-context.md

## Summary

A CLI-created logical session reports all startup sections as one opaque system prompt even when the assembler knows each section. Direct Prompt consumers also missed the confirmed receipt although the ledger retained it.

## Reproduction

- **Charter:** CH-session-context-provenance · **Tour:** Money
- **Environment:** isolated session-context lab, ACP mock subprocess, real SQLite, CLI/HTTP/UDS

1. Create a session through `compozy session new` using a fixture-backed agent with a skills catalog.
2. Send its first prompt and read `session usage --turns -o json` and the HTTP usage endpoint.
3. Compare the startup receipt with the known delivered catalog. The skills row says it is included in the startup prompt instead of retaining its measured owner.

**Expected:** Known startup section ownership survives the lazy runtime bind. Prompt consumers receive confirmed delivery before done.
**Actual:** The lazy bind reused the bound prompt text but lost its manifest; a separate Prompt output allowlist omitted the receipt event.

## Evidence

- Initial public read: lab `qa/reported-http.json` and `qa/reported-turns-http.json`.
- Red integration log: `.cache/session-context/task06-harness-integration.log`.
- Green direct receipt integration: `.cache/session-context/task06-harness-integration-recheck.log`.
- Green logical bind integration: `.cache/session-context/task06-logical-startup-integration-recheck.log`.

## Fix

- **Root cause:** Two boundary omissions: `isPromptOutputEventType` and `preparePromptRuntimePlan` did not carry the new receipt/manifest data.
- **Fix commit:** included in the enclosing session-context delivery commit; current tree verified.
- **Regression test:** Existing `TestHarnessContextIntegrationMeasuresDeliveredSkillCatalogs` now asserts direct and logical first bind, dedup, changed catalog, stopped readback and hook replacement. Its fixture selects no model because the receipt-only driver advertises no model option.
- **Implementation:** Added receipt to the output allowlist; retained an independent startup manifest with the in-memory agent definition and runtime rollback snapshot. No storage shape changes.

## Verification

Integration replay and fresh public CLI/HTTP/UDS replay passed. Full startup owners are preserved in `qa/fixed-reported-http.json`; final payload refactor reran the real integration successfully. The fix is verified in the working tree; enclosing commit pending.
