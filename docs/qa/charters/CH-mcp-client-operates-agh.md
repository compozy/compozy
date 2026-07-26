# CH-mcp-client-operates-agh: An unmodified MCP client runs one workspace end to end

```yaml
charter:
  id: CH-mcp-client-operates-agh
  mission: "As Ada driving a third-party MCP client, operate one workspace through agh mcp serve — list, create, verify natively — and prove auth, isolation, and the untouched native registry digest."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-agh-from-mcp-client
  scenarios: [ET-workspace-host-api-mcp]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Spawn agh mcp serve --workspace A over stdio from a real third-party MCP client; list tools, list sessions, create one session and one task; verify both through the native HTTP API."
      - "Bind a relay to workspace B and prove workspace A data is unreachable (same shape as not-found)."
      - "Loopback HTTP: tokenless and wrong-token connections reject deterministically; the exact env-sourced token connects; non-loopback bind refused at startup."
      - "Diff the native registry digest (zero new agh__* IDs) and confirm process exit leaves no relay listener or façade principal."
    must_avoid:
      - "Custom client shims — the client must stay unmodified; that is the story."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
