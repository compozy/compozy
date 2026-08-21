# CH-extension-agent-observation: Refresh authorization after an agent catalog change

```yaml
charter:
  id: CH-extension-agent-observation
  mission: "As Ada, change a resource-defined agent and prove observation uses the latest authorization while stopped events retain persisted session truth."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-dev-lifecycle
  scenarios: [ET-extension-agent-observer-resolution]
  tour: Feature Tour
  time_box_minutes: 35
  guidance:
    must_try:
      - "Run global, workspace, and builtin agent resolution with and without workspace context."
      - "Change provider authorization in the resource catalog and verify the next observation refreshes."
      - "Stop the session, emit a late event, and compare model, permission, and authorization observations from a second workspace."
    must_avoid:
      - "Restarting the daemon between catalog revisions, which would hide cache invalidation defects."
      - "Accepting logs or direct metadata files as the only evidence."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
