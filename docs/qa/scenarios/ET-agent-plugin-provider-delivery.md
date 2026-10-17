---
id: ET-agent-plugin-provider-delivery
area: ET
title: Deliver one ingested package through supported managed providers
persona: Ada
journey: J-extension-distribution
expected: "A real Claude Code session consumes the enabled Agent Plugins instance: the ingested skill activates, the stdio MCP server returns its absolute PLUGIN_ROOT and writable PLUGIN_DATA environment contract, and the streamable-HTTP server is reached through the daemon with no remote credential or package-format projection into the provider. A session on a provider configured with `session_mcp = false` that requires hosted MCP fails before launch instead of starting without its declared tools."
entry_points: compozy session new --workspace <workspace> --agent <claude-code|session-mcp-disabled custom provider>; compozy session prompt <session-id> <prompt>; compozy session events <session-id>; POST /api/sessions and session prompt/events over HTTP and UDS
qa_status: pass
bug_ids: BUG-20260816-hosted-mcp-bootstrap-projection; BUG-20260816-openclaw-session-mcp-gap
fix_status: fixed
retest_status: pass
fix_commits: 35100d40b55c
evidence: docs/qa/evidence/2026-08-16-agent-plugins/provider-matrix.json
last_report: docs/qa/reports/2026-08-16-agent-plugins.md
overlaps: ET-agent-plugin-conformance-walk; ET-agent-plugin-source-install; ET-managed-session-skill-loading
---

Added by Agent Plugins task 07 as the provider-neutral delivery evidence gate. Task 08 must use real
Claude Code, OpenClaw, and Hermes sessions against one installed generation and record a three-row
matrix at `docs/qa/evidence/2026-08-16-agent-plugins/provider-matrix.json`. Every row includes the
provider, session id, skill-activation observable, stdio environment/data-write observable, remote
server observable, redaction scan, and evidence paths. Retry-only success must disclose the first
attempt; provider-specific package loading does not satisfy this scenario.

QA 2026-08-16: Claude Code and Hermes each loaded the shared skill and invoked the canonical stdio and
remote tools on the first final-code attempt. OpenClaw failed before launch because its current ACP
bridge rejects per-session MCP servers; CompozyOS's `session_mcp=false` capability is truthful. The user
resolved the product decision by limiting the portable MCP delivery claim to Claude Code and Hermes.
The same recorded walk therefore passes the narrowed contract without rewriting the OpenClaw result.

QA impact 2026-10-17 (issue #761, long-tail provider removal): `hermes` and `openclaw` are no
longer built-in providers, so the portable delivery claim is Claude Code only and the hosted-MCP
refusal is now the generic `session_mcp = false` path. The next walk uses a real Claude Code session
plus a custom `[providers.<id>]` entry with `session_mcp = false`; the 2026-08-16 Hermes and OpenClaw
rows above stay as historical evidence. Status stays pass: the Claude Code delivery path is unchanged
by #761 and keeps its 2026-08-16 proof, and the generic `session_mcp = false` refusal is pinned by the
`internal/session` suite (`TestCreateSkipsHostedMCPWhenProviderDisablesSessionMCP` and the hosted-MCP
fail-before-launch case) against a custom provider.
