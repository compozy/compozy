# QA Run Report — 2026-08-05 — Issue 312 review remediation

- **Scope:** Concurrent Cursor bootstrap, atomic catalog generations, exact runtime-ID acceptance, and exact-ID Web interaction.
- **Cadence tier:** targeted
- **Build:** `codex/issue-312-cursor-model-catalog` working tree · **Environment:** isolated lab `issue-312-review-remediation-final-20260805-230015-520918`, daemon `http://127.0.0.1:54529`
- **Started:** 2026-08-05T23:01:26Z · **Completed:** 2026-08-05T23:14:46Z · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Autonomous Agent | desktop / wifi-fast / en-US | CH-cursor-account-models |
| Bruno | Delivery Builder | desktop / wifi-fast / en-US | CH-web-exact-model-id |
| Dora | Detail Debugger | desktop / wifi-fast / en-US | CH-compozy-runtime-input-preflight |

## Flows in Scope

- `J-20` — discover and inspect the truthful provider catalog from structured surfaces.
- `J-17` — choose an exact or provisional custom model ID in the composer.
- `J-02` — preserve an exact model ID in effective runtime configuration.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-cursor-account-models | J-20 / MS-cursor-account-model-discovery, MS-042, MS-055 | Ada | Feature Tour | PASS | — | — |
| 2 | CH-web-exact-model-id | J-17 / RT-web-exact-model-id-entry, RT-065 | Bruno | Feature Tour | PASS | — | — |
| 3 | CH-compozy-runtime-input-preflight | J-02 / LP-runtime-validation-preflight | Dora | Garbage Tour | PASS | — | — |

## Session Debriefs

- **Ada:** simultaneous first CLI/UDS and HTTP reads returned the same 193 real Cursor account rows,
  exact IDs, and one generation timestamp. Cached read preserved that timestamp; explicit refresh advanced it.
- **Bruno:** exact entry focused its labelled field. Cancel restored catalog search focus. Enter and
  pointer confirmation both returned to the open catalog with search focused. Ordinary unmatched
  search still selected `Direct-Custom-ID` without entering exact mode.
- **Dora:** CLI, HTTP, and UDS dry-runs exposed `cursor/composer-2.5` in the effective worker config.
  Unknown provider remained a typed error, and the dry-runs created zero runs.

## Paper Cuts

None.

## Runtime Errors Observed

An initial lab was intentionally abandoned before the journey because a readiness probe consumed the
first catalog read. It was not reused; its teardown was clean. No product error occurred in the final lab.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Final Status

**Verdict: PASS.** Evidence is indexed in the isolated lab's `qa/issue-312-review-evidence.md`.
Both the abandoned setup lab and the final evidence lab have machine-owned `teardown.json` files with
`"clean": true`; the final lab stopped registered daemon PID 76693 and Web PID 79779 with no survivors.
The strict evidence audit is repeated after the final full gate; its report and current gate record are
the authoritative final verification evidence.
