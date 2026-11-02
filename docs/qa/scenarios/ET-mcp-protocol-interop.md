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

Hosted-provider late discovery regression (#1120): after a managed provider has completed its first
`tools/list`, finish discovery of a trusted MCP source. Require the hosted server to advertise
`tools.listChanged`, emit `notifications/tools/list_changed`, and return the late tools on the next
list. Call a late tool through the provider. Cover the provider's `2025-06-18` protocol as well as
modern subscribed clients; cache hints alone cannot refresh legacy clients. Replay identical
projection snapshots across fallback refreshes and require no additional change notifications.
Keep the provider connected and preserve session/workspace policy filtering.

Targeted verification 2026-11-01: the existing hosted proxy suite exercises real MCP protocol pipes,
checks notification delivery and refreshed discovery for modern and provider legacy clients, and
checks silence on repeated snapshots (including a new digest with identical descriptors).
The prior broader interoperability scenario remains skipped; these focused checks do not establish
its HTTP isolation or unsupported-capability outcomes.

For a large hosted projection, configure two read-only MCP sources with 35 and 79 tools and delay
upstream discovery. Require complete operator and session projections with discovery work bounded
by the number of sources, rather than multiplied by the tool count. Withdraw one source's auth
and require its tools to disappear from hosted discovery on the next refresh. Availability batching
is request-local: it must not retain authorization or descriptors across refreshes or workspaces.

A direct Claude ACP adapter 0.99.1 probe completed the early tool call, then discovered and called
a tool registered only after that first call, returning `LATE_TOOL_REACHED`. The probe used the
production hosted proxy over stdio and a synthetic daemon-client boundary; it does not reproduce
the reporter's remote services or establish that their original environment is repaired.

The delayed-server race probe returned all 114 tools in 0.76 s for the operator and 0.73 s for the
session; failing the 79-tool source retained all 35 healthy tools. Before batching, the same
operator list took 36.04 s. These synthetic upstreams establish the repeated-discovery defect;
they do not identify the cause in the reporter's original services.
