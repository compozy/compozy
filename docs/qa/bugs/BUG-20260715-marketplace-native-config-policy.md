# BUG-20260715-marketplace-native-config-policy: Agent config tool cannot manage Marketplace catalog timing

- **Status:** verified
- **Impact (user-side):** Blocks agent-manageable configuration
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada; Vera
- **Journey Step:** J-extension-policy-admin, validate and apply catalog timing
- **Scenarios:** MS-marketplace-catalog-live-config; ET-049
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-15-marketplace.md
- **Origin:** Task 11 isolated Marketplace QA

## Summary

The operator CLI could apply `marketplace.catalog.ttl` and `marketplace.catalog.timeout` live, but the registered `agh__config_set` tool rejected both paths as `config_path_forbidden`. Agents could inspect and refresh Marketplace state but could not manage its non-sensitive fetch timing through the canonical config tool.

## Reproduction

1. Resolve the live `agh__config_set` descriptor.
2. Invoke it with `{"path":"marketplace.catalog.ttl","value":"0s"}` or a valid duration.
3. Inspect the structured tool decision and the applied config.

**Expected:** `ttl` and `timeout` reach canonical config validation; invalid durations return `config_validation_failed` without a write, and valid durations apply live. The feed `base_url` remains an operator-only trust root.
**Actual:** Every `marketplace.catalog.*` path returned the generic `config_path_forbidden` denial before validation.

## Evidence

- Live pre-fix invocation returned `tool_denied` with `config_path_forbidden` for `marketplace.catalog.ttl`.
- The canonical path-policy and native-tool tests failed red before the allowlist and trust classification changed.
- Final isolated-lab evidence is indexed by `docs/qa/reports/2026-07-15-marketplace.md`.

## Fix

- **Root cause:** The agent-facing typed mutation registry did not include the two Marketplace duration paths, and the Marketplace feed root had no explicit trust-root classification.
- **Correction:** `marketplace.catalog.ttl` and `.timeout` are typed duration mutations. `marketplace.catalog.base_url` is explicitly classified as a trust root and remains operator-only.
- **Fix commit:** pending Phase D checkpoint
- **Regression test:** The canonical config path-policy table covers both allowed duration paths and the denied feed root. The existing native-tool mutation suite proves a valid duration persists, an invalid duration preserves the prior value, and the feed root returns `config_trust_root_forbidden`.

## Verification

- `go test -race ./internal/config -run TestToolConfigPathPolicy -count=1`: 66 passed.
- `go test -race ./internal/daemon -run 'TestDaemonNativeTools/Should_mutate_allowed_config_paths_and_reject_guarded_config_paths' -count=1`: 8 passed.
- A rebuilt isolated daemon returned `config_validation_failed` for `ttl=0s` while preserving `5s`, denied `base_url` with `config_trust_root_forbidden`, and applied `timeout=3s` with `lifecycle=live` and `next_action=none`.
