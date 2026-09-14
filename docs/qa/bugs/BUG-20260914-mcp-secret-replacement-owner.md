# BUG-20260914-mcp-secret-replacement-owner: An invalid MCP credential replacement removes the working secret

- **Status:** verified
- **Impact (user-side):** Data-Loss
- **Severity:** Critical
- **Persona Affected:** Bruno
- **Journey Step:** J-mcp-authorize-repair, replace an OAuth client-secret reference
- **Scenarios:** ET-cli-mcp-authorize
- **Report:** docs/qa/reports/2026-09-14-marketplace-review-public.md

## Reproduction

CH-marketplace-public-ownership, Data Tour: configure an owned MCP client secret, then use Settings PUT to bind a secret owned by another profile. PUT returns 200 and deletes the previous owned secret. Auth begin later returns 500; CLI auth status exposes the ownership rejection. Vault metadata reports the former owned credential absent. Evidence: `docs/qa/evidence/2026-09-14-marketplace-review-public/step-119.json` through `step-122.json`.

## Fix

Settings now normalizes and validates the configured OAuth client-secret owner after preparing any supplied new value but before taking cleanup snapshots, storing secrets, or replacing the definition. It reuses the same Vault owner policy as auth resolution, retaining support for own, shared, environment and released user references.

Owning invariant: an invalid configured client-secret owner cannot replace a working MCP definition or remove its credential. Extended the existing `TestMCPSecretValuesStoreVaultSecrets/Should_store_OAuth_client_secret_values_without_writing_plaintext_config` case with foreign-profile and daemon-owned access/refresh/DCR/registration refs. It checks validation identity, exact unchanged config bytes and preserved credential value after every rejection. Focused race suite passed in 1.250s. Existing test-shape heuristic findings are byte-identical to the committed file; none originates in the changed subtest. Public replay is recorded below; final-head CI remains pending.

Public replay: 2422a5793, steps 126–141: five invalid replacements reject with unchanged config and retained secret; restart, auth status, and exact preview/apply deletion counts pass.
