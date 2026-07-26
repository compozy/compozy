# BUG-20260715-mcp-install-null-values: Input-free curated MCP installs reject the Web request

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-marketplace-acquisition, guided remote MCP installation
- **Scenarios:** ET-web-mcp-guided-install; ET-api-mcp-catalog-install
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-15-marketplace.md
- **Origin:** Task 11 isolated Marketplace QA

## Summary

The Marketplace guided installer correctly submitted `values: null` for a remote OAuth catalog entry with no environment or secret inputs, but the API parser rejected the request with `mcp-servers.install.values is required`. The service already owned catalog-field requirements and accepted an empty value set for input-free entries, so the transport introduced a false mandatory field that blocked the public Web flow.

## Reproduction

1. Publish a curated remote MCP entry with OAuth metadata and no environment fields.
2. Open `/marketplace/mcp/<entry-id>` and select **Install** without adding values.
3. Observe the API response and the Marketplace card.

**Expected:** The structural install succeeds, preserves catalog provenance, reports `next_step=authorize`, and appears on `/mcp`.
**Actual:** The API returned HTTP 400 with `settings validation error: mcp-servers.install.values is required`; no install was created.

## Evidence

- Pre-fix canonical API owner test reproduced the HTTP 400 for `values: null`.
- Isolated retest screenshot: `/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/screenshots/marketplace-guided-oauth-installed.png`.
- Authorization and workspace-isolation note: `/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/notes/mcp-guided-oauth-workspace-isolation.json`.

## Fix

- **Root cause:** `parseInstallSettingsMCPServerRequest` rejected a nil `values` pointer before the catalog service could validate entry-owned required inputs. This contradicted the contract's optional pointer and the Web request builder's intentional null for input-free entries.
- **Correction:** The parser now maps a nil pointer to the zero `MCPCatalogInstallValues`; catalog-specific required inputs remain validated by the settings service.
- **Fix commit:** pending Phase D checkpoint
- **Regression test:** The existing canonical API install suite now proves `values: null` reaches the service as an empty value set. It failed before the parser correction and passes afterward.

## Verification

- `go test -race ./internal/api/core -count=1` passed.
- A rebuilt isolated daemon accepted the unchanged Web request, returned the guided install, preserved `catalog_entry=qa-guided-oauth` and `catalog_version=1.0.0`, and exposed **Manage** to `/mcp`.
- The following browser OAuth flow reached `authenticated && token_present`; the deliberately unreachable fake MCP runtime remained independently `runtime_unavailable` with `probe=failed`.
