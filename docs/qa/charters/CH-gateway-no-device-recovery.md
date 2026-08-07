# CH-gateway-no-device-recovery: Recover with no paired device

```yaml
charter:
  id: CH-gateway-no-device-recovery
  mission: "As Iris, run the Back-Button Tour after the last paired device is revoked or lost, and prove every remote route stays closed while the local daemon surface can mint an accessible replacement pairing without reviving any old identity."
  mode: charter-with-tour
  persona:
    name: Iris
    device: laptop
    network: wifi-slow
    locale: en-US
  journey: J-expose-and-pair-gateway
  scenarios: [RT-gateway-no-device-recovery, RT-gateway-paired-device, RT-gateway-public-ui-consent]
  tour: Back-Button Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Revoke the only device, then use Back, Forward, refresh, a bookmarked deep link, and a second public/private context; none may reveal cached product data or a pairing action."
      - "From the daemon host, inspect the empty inventory and mint a replacement; confirm QR and copyable text agree and keyboard/screen-reader users can obtain the artifact."
      - "Let one artifact expire and consume another twice before completing a fresh pairing; errors must not reveal whether a guessed artifact ever existed."
      - "Confirm the replacement has a new identity and every old credential remains terminal after daemon restart."
    must_avoid:
      - "Assuming a hosted account recovery path exists; copying credential files as recovery evidence."
  evidence_expectations:
    - "Remote gate/access-ended states, local empty inventory, QR/text equivalence, expired/reused refusal, replacement device record, and rejected old credentials."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->

