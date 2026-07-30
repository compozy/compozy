# CH-untested-019-19-lea: Settle J-19 for Lea

```yaml
charter:
  id: CH-untested-019-19-lea
  mission: "Walk J-19 as Lea and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Lea
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-19
  scenarios: [RT-onboarding-setup-panel-over-shell]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: First-run setup renders over an inert desktop shell."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
