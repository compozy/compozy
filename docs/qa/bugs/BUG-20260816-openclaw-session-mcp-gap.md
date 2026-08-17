# BUG-20260816-openclaw-session-mcp-gap: OpenClaw cannot receive portable MCP tools

- **Status:** closed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-extension-distribution, consume one package through OpenClaw
- **Scenarios:** ET-agent-plugin-provider-delivery
- **Found:** 2026-08-16 · **Report:** docs/qa/reports/2026-08-16-agent-plugins.md

## Summary

The authored Agent Plugins spec promised the same ingested MCP resources through Claude Code,
OpenClaw, and Hermes, but OpenClaw's current ACP bridge rejects per-session MCP servers. CompozyOS
truthfully marks that provider as not supporting session MCP, so the required hosted projection fails
closed before a provider turn starts.

## Reproduction

- **Charter:** CH-agent-plugin-provider-delivery · **Tour:** Feature Tour
- **Environment:** macOS arm64, fresh isolated lab, enabled `acme.tools` package

1. Create an OpenClaw-managed session whose agent requests the portable stdio and remote tools.
2. Prompt the session.

**Expected:** OpenClaw consumes the same daemon-hosted tools as Claude Code and Hermes.
**Actual:** Session `sess-36b29c3eab89472f` failed with `required hosted MCP is unavailable: provider openclaw disables session MCP` before provider launch.

## Resolution

On 2026-08-16, the user limited the portable MCP delivery promise to the complete paths verified on
Claude Code and Hermes. OpenClaw remains a documented provider limitation until its ACP bridge accepts
the required session resources. Flipping CompozyOS's capability flag is not a fix because the provider
rejects the configuration.

## Verification

- **Result:** Pass for the narrowed contract — the provider matrix records Claude Code and Hermes
  delivery, while OpenClaw fails closed at the truthful capability boundary.
- **Evidence:** `docs/qa/evidence/2026-08-16-agent-plugins/provider-matrix.json`;
  `internal/config/provider_builtin.go`; `packages/site/content/docs/extensions/agent-plugins.mdx`;
  https://docs.openclaw.ai/tools/acp-agent.
