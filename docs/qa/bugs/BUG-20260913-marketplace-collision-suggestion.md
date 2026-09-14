# BUG-20260913-marketplace-collision-suggestion: Collision notice names the suggested source as occupied

- **Status:** verified
- **Impact (user-side):** Paper-Cut
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Bruno
- **Journey Step:** J-marketplace-acquisition, Add a plugin marketplace
- **Scenarios:** ET-web-marketplace-sources-add
- **Found:** 2026-09-13 · **Report:** docs/qa/reports/2026-09-13-marketplace-catalog.md

## Reproduction

With partner-plugins registered, preview another local folder named partner-plugins.
The dialog claims partner-plugins-2 is occupied while simultaneously suggesting that same name.
Expected: identify the occupied name when known, otherwise describe the collision without inventing it.
Evidence: ../evidence/2026-09-13-marketplace-catalog/add-name-collision.png.

## Fix and Verification

The formatter no longer substitutes suggested_name for an unknown occupied name. It uses the explicit
submitted name or a generic collision sentence; the suggested name remains in the recovery action.
No behavior or API change. Production rebuild passed; fresh live preview reports the generic occupied-name sentence and suggests partner-plugins-2 only as recovery. Screenshot add-name-collision-after.png inspected. Cancel preserved source inventory. Fix commit pending.
