# CH-agent-marketplace-parity: Manage sources and acquire extensions across structured planes

Mission updated for the approved Marketplace hard cut. Historical debriefs stay in their dated reports; task_10 owns the next run.

```yaml
charter:
  id: CH-agent-marketplace-parity
  mission: "Manage sources and acquire extensions across structured planes, with truthful origin, destination and persisted state."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-agent-marketplace-parity
  scenarios: [ET-cli-marketplace-sources, ET-api-marketplace-sources, ET-cli-marketplace-search, ET-cli-marketplace-info, ET-cli-marketplace-refresh]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Compare source list and catalog discovery on CLI, HTTP, UDS and their read-only native tools; validate experimental metadata and arrays."
      - "Dry-run, add, toggle, refresh and remove a local fixture source; compare structured errors and prove rejected requests leave registration and installation unchanged."
      - "Install a source plugin with its listed digest and compare inventory and origin across supported planes; retain baseline curated owner/repo selection and reject source-name takeover."
    must_avoid:
      - "Do not use retired per-kind Marketplace acquisition as a fallback or expose credentials in evidence."
```
