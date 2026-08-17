# CH-agent-plugin-dev-isolation: Reload portable content without crossing workspace boundaries

```yaml
charter:
  id: CH-agent-plugin-dev-isolation
  mission: "As Bruno, edit and reload one workspace-scoped Agent Plugins source while another workspace and the global installed instance remain isolated on their own generations and data."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-dev-lifecycle
  scenarios: [ET-agent-plugin-dev-reload]
  tour: Multi-Tab Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Create workspace A and B plus a global instance with the same dotted name; dev-link only A and compare status, inventory, resources, MCP behavior, and data paths on all three scopes."
      - "Reload valid changed bytes, then a fatal manifest and one invalid component; valid reload swaps atomically, fatal reload keeps last-good, and component failure skips only its sibling."
      - "Race two reload requests through separate public planes and prove the instance coordinator serializes them without torn generations or cross-workspace events/cache rows."
      - "Unlink and relink A, restart, and remove each identity; data and source-checksum generation behavior stay instance-scoped and deterministic."
    must_avoid:
      - "Treating the dev source as a trusted marketplace install, mutating workspace B to make assertions pass, or editing generated registry state directly."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
