# CH-gateway-provider-degradation: Degrade a provider without false reachability

```yaml
charter:
  id: CH-gateway-provider-degradation
  mission: "As Dora, run the Network Tour across provider authorization, endpoint verification, live health, and teardown, and prove every outage or route change removes reachability before any UI or structured surface can claim it is active."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: flaky
    locale: en-US
  journey: J-expose-and-pair-gateway
  scenarios: [RT-gateway-local-only-boot, RT-connectivity-provider-route, ET-connectivity-provider-trust, RT-gateway-operator-surface-truth]
  tour: Network Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Cancel authorization, deny provider permission, cut connectivity during establish, and return wrong-tier, missing-nonce, redirect, unstable, and TLS-invalid endpoints."
      - "After a verified route becomes live, change the endpoint set and stop the provider; observed state, address projection, ingress binding, and audit must move to degraded/off without stale green."
      - "Restore the provider under supervised retry and confirm re-advertising happens only after a fresh challenge for the correct tier."
      - "Compare web, CLI, HTTP, UDS, and events at each transition; provider inventory loading or failure must never render as an empty catalog."
    must_avoid:
      - "Trusting provider-reported health without client-path verification; using a stubbed provider as reachability evidence."
  evidence_expectations:
    - "Timestamped provider state transitions, challenge result, address presence/absence across planes, audit finding and remediation, and restored verified route."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->

