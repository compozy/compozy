# CH-bundled-extension-trust: Bundled spec-cycle remains verified under extension policy

```yaml
charter:
  id: CH-bundled-extension-trust
  mission: "As Vera, inspect the bundled spec-cycle extension in Web and structured surfaces, restart the runtime, and confirm first-party trust remains verified without changing side-load policy or lifecycle state."
  mode: scenario-based
  persona:
    name: Vera
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-policy-admin
  scenarios: [ET-bundled-extension-trust, ET-web-extension-detail, ET-web-extension-kit-inventory]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open spec-cycle from Extensions Installed and confirm no policy-block warning appears."
      - "Compare Web trust with `compozy extension provenance spec-cycle -o json` and the public extension API."
      - "Disable spec-cycle, restart the runtime, and confirm provenance is reconciled while disabled state and install time remain unchanged."
    must_avoid:
      - "Do not enable unverified side-loads or reinstall spec-cycle as an external extension."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
