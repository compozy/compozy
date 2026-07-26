# BUG-20260715-marketplace-stale-report: Failed feed refresh hides the preserved stale projection

- **Status:** fixed and verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Vera; Ada
- **Journey Step:** J-extension-policy-admin kill-switch fallback; J-agent-marketplace-parity refresh
- **Scenarios:** ET-marketplace-kill-switch; ET-cli-marketplace-refresh; ET-cli-marketplace-search; ET-api-marketplace-namespace
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-15-marketplace.md
- **Origin:** Task 11 isolated Marketplace QA

## Summary

The catalog service correctly retained rows, marked their kind stale, and returned a populated per-kind refresh report when the feed became unavailable. The public handler discarded that report whenever the service also returned its joined error, while search and browse mapping discarded the durable `KindState`. Operators received an opaque failed command or apparently fresh rows instead of the promised stale-marked fallback.

## Reproduction

1. Refresh a healthy isolated feed containing one MCP entry.
2. Point `marketplace.catalog.base_url` at an unreachable loopback port and refresh.
3. Search or browse the previously projected entry through CLI, HTTP, UDS, and Web.

**Expected:** Refresh returns per-kind `outcome=failed`, prior `entry_count`, `stale=true`, and `error_class`; browse/search keep the prior row and expose its stale diagnostic.
**Actual:** `agh marketplace refresh -o json` exited 69 with only the joined error. Search/browse served the retained row but omitted `stale`, `error_class`, and the redacted source diagnostic.

## Evidence

- Isolated lab note: `/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/notes/marketplace-kill-switch-stale-report.json`.
- Pre-fix API owner tests failed with HTTP 500 for the populated refresh report and missing stale fields on both grouped search and kind browse.
- The internal Marketplace integration suite already proved the service/store retained and marked the projection; the missing boundary was public mapping.

## Fix

- **Root cause:** `RefreshMarketplaceCatalog` treated any joined service error as an error response even when `RefreshReport.Outcomes` was complete. `curatedMarketplaceKind` ignored `BrowseResult.State`, and `MarketplaceKindResponse` had no stale-state fields.
- **Correction:** A populated refresh report is now the successful structured response even when individual outcomes failed. Grouped and kind responses co-ship required `stale` plus optional `error_class` and redacted `error` from the service state through HTTP, UDS, CLI, native discovery, generated OpenAPI/TypeScript, Web, site docs, and the official AGH skill.
- **Fix commit:** pending Phase D checkpoint
- **Regression test:** Existing API core Marketplace suites now prove stale rows remain visible with their state and that failed refresh outcomes return HTTP 200; the canonical OpenAPI suite owns the required/optional field shape.

## Verification

- Targeted API core/spec and native-tool tests pass under `-race`.
- A rebuilt isolated daemon returns exit 0 with three failed/stale per-kind outcomes; focused `--kind mcp` returns one failed/stale outcome with `entry_count=1`.
- CLI, HTTP, and UDS search/browse all retain the MCP row and report `stale=true`, `error_class=network`; workspace HTTP/UDS retain `installed=true` and the exact MCP management path.
- `--kind bundle` still exits 65 as a derived catalog and does not mutate the stale MCP projection.
- The final daemon-served Web bundle preserves the installed `qa-guided-oauth` card on both the Marketplace landing and the MCP kind view, presents one truthful stale warning per scope, and keeps the displayed count at one. Deterministic 1440×900 captures: `qa/screenshots/marketplace-stale-served-landing.png` and `qa/screenshots/marketplace-stale-served-kind.png` in the isolated lab.
- Restoring `marketplace.catalog.base_url` to the healthy feed applies live without restart; a fresh refresh reports all three feed-backed kinds `outcome=succeeded`, `stale=false`, and MCP `entry_count=1`. A subsequent CLI search retains the row with `stale=false`.
