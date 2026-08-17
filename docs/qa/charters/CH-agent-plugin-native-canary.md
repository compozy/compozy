# CH-agent-plugin-native-canary: Keep the adjacent native extension install path intact

```yaml
charter:
  id: CH-agent-plugin-native-canary
  mission: "As Bruno, install, invoke, inspect, and remove one native extension after the portable-layout changes, proving root selection and the established lifecycle remain unchanged."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-distribution
  scenarios: [ET-extension-published-source-installs]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Install the existing public native fixture from its immutable source, confirm format compozy and native resources, invoke its probe, and remove it through public surfaces."
      - "Validate the same native identity before install and prove no portable format marker, diagnostic, or resource appears on its lifecycle reads."
      - "Compare the native result with the prior scenario contract and record the canary evidence independently from portable success."
    must_avoid:
      - "Expanding into the full native publication journey, changing the public fixture, or counting a dual-invalid root as a successful native install."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
