# CH-untested-061-keep-secrets-contained-dora: Settle J-keep-secrets-contained for Dora

```yaml
charter:
  id: CH-untested-061-keep-secrets-contained-dora
  mission: "Walk J-keep-secrets-contained as Dora and settle the assigned current behaviors through their real public entry points."
  mode: scenario-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-keep-secrets-contained
  scenarios: [ET-web-vault-overwrite-confirmation, MS-038, MS-040, MS-041]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Execute every assigned scenario from its named entry point; capture command, timestamp, output, and end state."
      - "Exercise one recovery or abandonment branch and prove that it leaves no partial or duplicate side effect."
      - "Compare the owning public surfaces wherever the scenario promises Web, CLI, HTTP, UDS, or native parity."
      - "Prioritize these representative observables first: Vault create warns and requires confirmation before overwriting an existing ref; Vault list secrets; Vault store secret."
    must_avoid:
      - "Do not infer a pass from source, mocks, historical evidence, or an automated suite alone."
      - "Do not perform live publication or mutate scenarios outside this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
