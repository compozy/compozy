# CH-untested-063-marketplace-acquisition-bruno: Settle J-marketplace-acquisition for Bruno

```yaml
charter:
  id: CH-untested-063-marketplace-acquisition-bruno
  mission: "Walk J-marketplace-acquisition as Bruno and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-marketplace-acquisition
  scenarios: [ET-web-marketplace-installed-management, ET-web-marketplace-kind-navigation, ET-web-marketplace-remove-scope-return, ET-web-page-content-gutter, ET-web-route-chrome-topbar, ET-web-vault-opendesign-listing]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Manage every installed Marketplace kind; Navigate Marketplace kinds from the default entry; Return removed items to Marketplace scope."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
