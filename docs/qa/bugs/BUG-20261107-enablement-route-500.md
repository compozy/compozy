# BUG-20261107-enablement-route-500: Retry turning on shows Internal Server Error instead of the enable failure

- Status: fixed
- Fixed in: `68d106e25`
- Impact (user-side): Misleading error
- Severity: Medium; Priority: P2
- Persona Affected: Bruno
- Journey Step: Retry on a row that reads Couldn't turn on
- Scenarios: ET-agent-plugin-marketplace-install
- Origin: spec marketplace-plugin-catalog S8 integrated walk (isolated lab, 2026-11-07)

## Reproduction

With an installed extension whose enable fails validation (see
BUG-20261107-derived-secret-env-enable-fails), press **Retry** in Marketplace. The Web calls
`PUT /api/extensions/demo-api/enablement {"enabled":true,"profile":"default"}`, which returns
`500 {"error":"Internal Server Error"}`; the toast reads "Internal Server Error" twice. The CLI
(`compozy extension enable demo-api`) shows the real validation message.

Expected: a 4xx with the validation code/message, so the row and toast say why it couldn't turn on.
