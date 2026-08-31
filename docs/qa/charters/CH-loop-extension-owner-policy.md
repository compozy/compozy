# CH-loop-extension-owner-policy: Keep extension Loop actions within their owner

```yaml
charter:
  id: CH-loop-extension-owner-policy
  mission: "As Bruno, run an extension-owned Loop before and after daemon restart and prove that only same-owner action tools bypass the disabled external-source default."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-loop-extension-actions
  scenarios: [LP-extension-owned-loop-tool-policy]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Install two non-bundled, non-dev-linked test extensions without extra source grants."
      - "Run the first extension Loop with its own tool while tools.policy.external_default remains disabled."
      - "Reference the second extension tool from the first Loop and capture the structured denial."
      - "Restart the daemon, reload the persisted Run snapshot, and repeat the same-owner check."
    must_avoid:
      - "Do not enable tools.policy.external_default or add an operator source grant."
      - "Do not use bundled or development-linked trust behavior as evidence for marketplace ownership."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
