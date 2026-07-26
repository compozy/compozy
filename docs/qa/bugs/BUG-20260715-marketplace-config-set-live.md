# BUG-20260715-marketplace-config-set-live: Marketplace catalog config cannot be applied live through the CLI

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Vera
- **Journey Step:** J-extension-policy-admin, redirect discovery to an isolated curated feed
- **Scenarios:** MS-marketplace-catalog-live-config; ET-marketplace-kill-switch
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-15-marketplace.md
- **Origin:** Task 11 isolated Marketplace QA

## Summary

The documented `agh config set marketplace.catalog.*` surface rejected all three catalog keys as unsupported. After the CLI accepted the keys, the daemon still classified the reload as restart-required because its active/desired config diff omitted the Marketplace catalog entirely. Operators could not redirect discovery to an isolated feed or apply feed timing changes live as promised.

## Reproduction

1. Start an isolated daemon and a valid local Marketplace catalog feed.
2. Run `agh config set marketplace.catalog.base_url <feed-url> -o json`.
3. After accepting the path, change `marketplace.catalog.ttl` or `.timeout` while the daemon remains running.
4. Inspect the structured lifecycle result and refresh the Marketplace catalog.

**Expected:** Every catalog key is accepted with its declared scalar type, reports `lifecycle=live`, advances the active generation without a restart, and affects the next catalog refresh.
**Actual:** The initial command failed with `config path "marketplace.catalog.base_url" is not supported by config set`. With only the CLI registry corrected, the daemon persisted subsequent values but returned `restart-required` and left the active generation unchanged.

## Evidence

- Isolated lab note: `/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/notes/marketplace-config-set-live.json`
- Pre-fix CLI owner tests failed for `base_url`, `ttl`, and `timeout` in both path classification and the public config-set lifecycle.
- Pre-fix Settings owner test returned no changed paths for a config whose three Marketplace catalog fields all changed.

## Fix

- **Root cause:** The CLI scalar mutation registry omitted `marketplace.catalog.base_url`, `.ttl`, and `.timeout`. Its lifecycle adapter also routed config paths through UI Settings sections, even though the Marketplace catalog has no Settings section. Independently, `reloadChangedPaths` never compared `Config.Marketplace.Catalog`, so the daemon conservatively promoted every live catalog reload to restart-required.
- **Correction:** A dedicated CLI path-kind owner registers the three typed keys; the CLI lifecycle adapter now reads the canonical config lifecycle matrix directly; and a dedicated reload-diff owner emits each changed Marketplace catalog path before daemon reconciliation.
- **Fix commit:** pending Phase D checkpoint
- **Regression test:** Existing canonical CLI classification/lifecycle tables cover all three keys, while the existing Settings reload-diff suite proves the three canonical paths are emitted together. Both failed before their production owner was corrected.

## Verification

- `go test -race ./internal/cli -count=1` passed.
- `go test -race ./internal/settings -count=1` passed.
- A rebuilt daemon accepted `marketplace.catalog.timeout=5s` with `lifecycle=live`, `applied=true`, `restart_required=false`, and advanced active generation 2 → 3 without another restart.
- The immediately following `agh marketplace refresh -o json` loaded the isolated feed successfully with one MCP entry and non-stale outcomes for every feed-backed kind.
