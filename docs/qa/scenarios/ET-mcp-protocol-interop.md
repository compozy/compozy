---
id: ET-mcp-protocol-interop
area: ET
title: Interoperate with MCP 2026 peers without transport leakage
persona: Ada
journey: J-mcp-protocol-interop
expected: An unmodified official-SDK client uses Compozy over stdio or Streamable HTTP at 2026-07-28, negotiates 2025-11-25 only through the SDK, sees cacheScope=private and ttlMs on tool discovery, and receives typed errors for unsupported capabilities without SSE or cross-target authorization/cache leakage.
entry_points: official-SDK MCP stdio client spawning compozy mcp serve; official-SDK Streamable HTTP client; MCP status read
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md
last_report: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md
overlaps: ET-workspace-host-api-mcp; ET-compozy-native-tool-invocation
---

Skipped in the 2026-07-30 MCP 2026/catalog-v2 closeout: modern and SDK-negotiated legacy transports were observed, but cache isolation and unsupported-capability probes were not retained.

MCP 2026/catalog v2 planning flag. The future isolated walk must use the shared official-SDK
fixture profiles for modern, negotiated legacy, stateless HTTP, and unsupported-capability paths;
it must not use a compatibility shim or an SSE fallback.
