# BUG-20260914-profile-rename-mcp-reference: Renaming a profile disconnects its configured MCP credentials

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High
- **Persona Affected:** Bruno
- **Journey Step:** J-operate-profiles, rename a profile with an MCP client secret
- **Scenarios:** ET-profile-cli-lifecycle; ET-cli-mcp-authorize
- **Report:** docs/qa/reports/2026-09-14-marketplace-review-public.md

## Reproduction

CH-marketplace-public-ownership, Data Tour: save a profile-scoped HTTP MCP using the Settings API with a synthetic client secret. Rename `studio` to `creative` through the CLI. Vault metadata shows the new owned ref and no old ref, but the moved profile mcp.json still points to `vault:mcp/profile/studio/...`. MCP Settings reads return 500, and `mcp auth status --scope profile --profile creative` rejects the stale ref ownership.

Evidence: `docs/qa/evidence/2026-09-14-marketplace-review-public/step-100.json` through `step-108.json`. User-scoped reads and shared credentials remain present. The walk stopped before deleting the affected profile.

## Cause and required repair

Profile rename re-encrypts Vault rows and updates database ref occurrences, but its filesystem finalizer moves profile files without rewriting the configured references inside them. Reuse the durable lifecycle finalization/recovery path to update affected configured refs and include those occurrences in the preview. Preserve unrelated content, shared/user refs, path containment and retry safety. The existing real profile lifecycle suite must verify loaded config and resolvable credentials after rename, plus replay after an interrupted file finalization.

## Implemented repair

The existing rename-folder finalizer rewrites owned ref values in profile config.toml/mcp.json and can replay without changing already-rewritten bytes. JSON keys, TOML comments and surrounding bytes are retained. The rename preview includes personal-profile file occurrences. Selected repository profile folders use the same rewrite and return any repair error through their existing per-folder outcome.

Regression: `TestManagerProfileLifecycle/Should_keep_configured_MCP_credentials_usable_after_profile_rename_and_finalizer_replay` loads TOML and JSON after rename, resolves personal and workspace-profile credentials through real Vault, and verifies idempotent finalization and an unchanged shared secret. Public re-walk remains pending.
