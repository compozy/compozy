# CH-managed-session-skill-loading: Prove native managed skill loading

```yaml
charter:
  id: CH-managed-session-skill-loading
  mission: "Prove that a real managed Codex session loads an omitted skill through compozy__skill_view and never through the operator CLI or a direct file read."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-load-skill-in-managed-session
  scenarios: [ET-managed-session-skill-loading]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Install a distinctive workspace skill and independently record its exact body as the operator."
      - "Delay the first provider startup beyond the hosted-MCP bind TTL and force the skill out of the managed session's truncated catalog."
      - "Capture the native compozy__skill_view call and exact result in persisted session events."
      - "Inspect the complete transcript for any compozy skill view command or direct skill-file read."
      - "Run every compozy skill command with managed environment markers and prove the supported-path guard leaves state unchanged."
      - "Read the same skill from an operator shell and compare the bodies byte-for-byte."
      - "Confirm no session, daemon, provider, socket, or watcher remains after the run."
    must_avoid:
      - "Do not add managed identity headers, capability sockets, or environment-based authorization."
      - "Do not infer a pass from source, mocks, historical evidence, or an isolated tool call."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
