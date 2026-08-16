---
id: ET-agent-plugin-provider-delivery
area: ET
title: Deliver one ingested package through Claude Code, OpenClaw, and Hermes
persona: Ada
journey: J-extension-distribution
expected: "Three real managed sessions, one per supported ACP provider, consume the same enabled Agent Plugins instance: the ingested skill activates, the stdio MCP server returns its absolute PLUGIN_ROOT and writable PLUGIN_DATA environment contract, and the streamable-HTTP server is reached through the daemon with no remote credential or package-format projection into the provider."
entry_points: compozy session new --workspace <workspace> --agent <claude-code|openclaw|hermes>; compozy session prompt <session-id> <prompt>; compozy session events <session-id>; POST /api/sessions and session prompt/events over HTTP and UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-agent-plugin-conformance-walk; ET-agent-plugin-source-install; ET-managed-session-skill-loading
---

Added by Agent Plugins task 07 as the provider-neutral delivery evidence gate. Task 08 must use real
Claude Code, OpenClaw, and Hermes sessions against one installed generation and record a three-row
matrix at `docs/qa/evidence/2026-08-16-agent-plugins/provider-matrix.json`. Every row includes the
provider, session id, skill-activation observable, stdio environment/data-write observable, remote
server observable, redaction scan, and evidence paths. Retry-only success must disclose the first
attempt; provider-specific package loading does not satisfy this scenario.
