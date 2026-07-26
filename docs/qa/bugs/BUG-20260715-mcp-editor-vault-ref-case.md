# BUG-20260715-mcp-editor-vault-ref-case: MCP editor lowercases case-sensitive Vault references

- **Status:** verified
- **Impact (user-side):** Displays and copies an invalid credential reference
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-mcp-authorize-repair, edit an installed server
- **Scenarios:** ET-web-mcp-remote-editor
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-15-marketplace.md
- **Origin:** Task 11 isolated Marketplace QA

## Summary

The MCP editor rendered persisted `vault:` references through the generic `MonoId` primitive, which lowercased every value for display and clipboard copy. A canonical ref ending in `QA_TYPED_TOKEN` was therefore presented and copied as `qa_typed_token`, even though the daemon had persisted the correct bytes.

## Reproduction

1. Install a guided MCP server with a typed secret key containing uppercase characters.
2. Open the installed server in `/mcp`.
3. Inspect and copy the generated canonical Vault reference.

**Expected:** The reference renders and copies byte-for-byte exactly as persisted.
**Actual:** The generic identifier presentation lowercases the case-sensitive environment-key segment.

## Evidence

- Pre-fix screenshot: `/Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/web/mcp-editor-vault-ref-lowercase-red.png`.
- Sanitized red/green replay: `/Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/mcp-editor-vault-ref-case.json`.
- Final screenshot: `/Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/web/mcp-editor-vault-ref-case-green.png`.

## Fix

- **Root cause:** `MonoId` unconditionally lowercased both its rendered and copied value, while the two MCP editor call sites carry byte-sensitive Vault references rather than cosmetic identifiers.
- **Correction:** `MonoId` now has an explicit opt-in `preserveCase` contract. The stdio secret binding and OAuth client-secret call sites opt in; generic IDs retain their existing lowercase behavior.
- **Fix commit:** pending Phase D checkpoint
- **Regression test:** The canonical UI primitive suite covers exact render and copy behavior, and the MCP editor owner covers both the stdio and OAuth call sites.

## Verification

- Fresh root-Turbo UI owner: 6 tests passed with clean stderr.
- Fresh root-Turbo Web owner: 2 tests passed.
- After a clean browser reload, the editor contained the exact uppercase reference, contained no lowercase variant, and its Copy action emitted the exact persisted bytes. Browser errors were empty and the console contained only the React DevTools development notice.
