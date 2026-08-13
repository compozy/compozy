# CH-home-preload-recovery: Keep Home reachable during query failure

```yaml
charter:
  id: CH-home-preload-recovery
  mission: "As Cora, open Home while workspace or status data is unavailable and prove navigation reaches an honest recoverable state instead of failing the route."
  mode: strategy-based
  persona:
    name: Cora
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-operate-home-dashboard
  scenarios: [RT-home-dashboard-zones]
  tour: Network Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open Home normally and capture the truthful empty or populated zones."
      - "Interrupt status or workspace availability, revisit Home, and confirm the route remains reachable with a visible recovery state."
      - "Restore connectivity, retry, refresh, and compare the rendered scope with the public status and workspace reads."
    must_avoid:
      - "Do not use source code or database state to declare recovery successful."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
