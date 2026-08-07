# CH-gateway-live-revocation: Revoke a device during live remote work

```yaml
charter:
  id: CH-gateway-live-revocation
  mission: "As Iris, run the Multi-Tab Tour while a paired device has live session, task, loop, bridge, extension, and window streams, then revoke it from the trusted local surface and prove no tab can finish in-flight work or retain cached product data."
  mode: charter-with-tour
  persona:
    name: Iris
    device: laptop
    network: wifi-slow
    locale: en-US
  journey: J-expose-and-pair-gateway
  scenarios: [RT-gateway-paired-device, RT-gateway-public-ui-consent, RT-gateway-browser-stream-reconnect]
  tour: Multi-Tab Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Open private and consented-public views in separate contexts, start live reads and one state-changing action, then revoke from the local device while both contexts are active."
      - "Confirm every SSE and WebSocket closes, an in-flight mutation is fenced before commit, and each context reaches access-ended without workspace, session, or cached query data."
      - "Refresh, go Back/Forward, restore the network, and attempt direct API calls with the revoked credential; no path may recover access or mint a pairing publicly."
      - "Compare device state and gateway events over web, HTTP, UDS, and CLI after revocation."
    must_avoid:
      - "Treating a generic timeout or 5xx as revocation; replacing the production flow with intercepted browser responses."
  evidence_expectations:
    - "Before/after device inventory, both remote terminal screens, stream closure records, rejected mutation result, and structured device/event reads."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->

