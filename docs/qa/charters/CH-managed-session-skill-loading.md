# CH-managed-session-skill-loading: Prove managed skill loading keeps its identity boundary

```yaml
charter:
  id: CH-managed-session-skill-loading
  mission: "Walk the managed Codex skill-loading path and prove that native and CLI reads work only within daemon-validated scope."
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
      - "Install a distinctive workspace skill and independently record its exact verified body as the operator."
      - "Load that body inside a managed Codex session through compozy__skill_view and through compozy skill view."
      - "Attempt a foreign-workspace read and a mismatched-agent read; capture the structured denial and prove the own-scope read still works afterward."
      - "Attempt to disable the skill through the managed CLI transport; capture the denial and prove a later read still works."
      - "Confirm no session, daemon, or provider process remains after the run."
    must_avoid:
      - "Do not set, replace, or remove daemon-issued COMPOZY_SESSION_ID, COMPOZY_AGENT, or COMPOZY_AGENT_TRANSPORT_SOCKET variables."
      - "Do not infer a pass from source, mocks, historical evidence, or automated tests alone."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
