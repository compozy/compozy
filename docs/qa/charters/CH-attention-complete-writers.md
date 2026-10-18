# CH-attention-complete-writers: Keep one complete attention policy across competing writers

```yaml
charter:
  id: CH-attention-complete-writers
  mission: "As Dora, submit two competing complete attention policies through public transports and prove both Settings tabs converge to one persisted candidate."
  mode: charter-with-tour
  cycle_tier: targeted
  cycle_role: continuation
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-administer-runtime-settings
  scenarios: [MS-attention-settings-roundtrip]
  tour: Multi-Tab Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open Notifications in two tabs, record global and profile baselines, and submit a bounded pair of complete global candidates over HTTP and UDS with overlapping request intervals."
      - "Observe live convergence within 25 seconds, then independently read HTTP, UDS, config CLI and config.toml; a whole candidate must win without mixed fields or changed profile mutes."
      - "Reload both tabs, restore the complete original policy and prove the same live runtime without a restart. Use no more than three paired attempts and stop on a divergence."
    must_avoid:
      - "No unrelated or background config writer may run alongside the deliberate pair. Do not inspect product internals, change browser permission, or infer audible or OS notification delivery from saved values."
```

<!-- Durable continuation mission; debriefs belong only in the dated report. -->
