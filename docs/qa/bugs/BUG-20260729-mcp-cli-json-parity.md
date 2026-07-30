# BUG-20260729-mcp-cli-json-parity: Workspace MCP CLI JSON diverged from daemon payloads

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** Install and authorize one workspace-scoped MCP server through structured planes
- **Scenarios:** ET-cli-mcp-install; ET-cli-mcp-authorize
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Origin:** Fresh isolated CLI/HTTP/UDS install and OAuth replay

## Summary

Workspace-scoped `mcp install`, `mcp authorize`, `mcp auth status`, and `mcp auth logout` JSON added
a CLI-only `resolution_source` field. The daemon's HTTP and UDS payloads did not contain that field,
so agents could not reuse one exact structured contract across planes.

## Reproduction

1. Install a curated MCP entry with `--scope workspace --workspace <id> -o json`.
2. Authorize an OAuth MCP definition in the same scope and read or revoke its status as JSON.
3. Compare each complete CLI response with the corresponding daemon-authored contract.

**Expected:** Structured CLI JSON preserves the daemon payload exactly; workspace-discovery
diagnostics are available only through an explicit contract.
**Actual:** Each workspace-resolving command added top-level `resolution_source: "flag"`.

## Evidence

- Install, automatic/manual OAuth, timeout, scope-isolation, redaction, and cleanup assertions:
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/023-mcp-catalog-install`.

## Fix

- **Root cause:** `writeCommandOutput` routes structured output through `writeJSON`, which augments
  every payload whenever workspace resolution was recorded on the Cobra command. MCP bundles do not
  opt into that field and expose daemon response types as their JSON contract.
- **Correction:** pending structural design. This is the third command family hit by the same generic
  writer behavior in this workstream, so the two-touch rule requires a TechSpec instead of another
  command-local `writeJSONWithoutWorkspaceResolution` patch.
- **Fix commit:** pending
- **Regression owner:** `internal/cli`; the structural suite must cover MCP install and auth plus the
  existing overview and Marketplace contract-preservation cases.

## Verification

- The install, OAuth, scope isolation, timeout, logout, Vault lifecycle, and redaction behaviors are
  green on the current candidate.
- **Retested:** parity fix not implemented; structural TechSpec decision pending
