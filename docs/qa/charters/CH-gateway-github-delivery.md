# CH-gateway-github-delivery: Deliver a GitHub webhook through Tailscale

```yaml
charter:
  id: CH-gateway-github-delivery
  mission: "As Bruno, connect a private GitHub repository to one Compozy Loop through the verified Tailscale public gateway, send a real push delivery, and confirm the GitHub receipt and exactly one attributed Loop run agree after fresh reads."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-deliver-through-public-gateway
  scenarios: [RT-gateway-public-ingress-bindings, TA-056]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Create a private disposable repository, configure its push webhook with the current projected URL and write-only secret, and make one normal commit through GitHub."
      - "Confirm the GitHub delivery receipt, refresh the Compozy trigger, and independently read the resulting Loop run with delivery attribution."
      - "Verify that a second delivery identity is not invented and that the trigger read never returns the webhook secret."
    must_avoid:
      - "Treating a local signed request as proof of public Tailscale routing or accepting only one side's receipt."
  evidence_expectations:
    - "Tailscale gateway status, trigger projection after refresh, GitHub delivery receipt, and the matching Compozy Loop run and history read."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
