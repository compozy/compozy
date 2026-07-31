# CH-mcp-protocol-interop: Prove a standard MCP client can use the supported protocol contract

```yaml
charter:
  id: CH-mcp-protocol-interop
  mission: "As Ada operating an unmodified official-SDK MCP client, run the Feature Tour through initialization, tool discovery, a supported call, an unsupported-capability refusal, and cache boundaries so protocol version and target identity remain truthful without an SSE or client-shim escape hatch."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-mcp-protocol-interop
  scenarios: [ET-mcp-protocol-interop, ET-workspace-host-api-mcp, ET-compozy-native-tool-invocation]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Use the shared official-SDK fixture as a 2026-07-28 peer, then its 2025-11-25 profile; record the negotiated version from the public status/read surface and prove no other compatibility version is accepted."
      - "Connect through stdio and Streamable HTTP, list tools twice around ttlMs, then change target or authorization binding; cacheScope must be private and no result may cross the boundary."
      - "Call one supported tool and independently read its effect; then request an unsupported MRTR/input-response capability and capture the typed refusal rather than a silent no-op or advertised support."
      - "Disconnect a client mid-flow and reconnect fresh; confirm no retained MCP transport session, private cache result, or obsolete SSE endpoint is required."
    must_avoid:
      - "Custom client shims, direct internal calls, synthetic cache mutation, or a fallback to SSE. The walk is only valid through public MCP interfaces."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
