# CH-gateway-remote-cli-interruption: Recover a remote CLI profile from interruption

```yaml
charter:
  id: CH-gateway-remote-cli-interruption
  mission: "As Iris, run the Interrupt Tour across encrypted profile transfer, explicit target selection, a live remote command, stream reconnect, revocation, and removal, and prove every failure preserves the right recovery path without exposing credentials."
  mode: charter-with-tour
  persona:
    name: Iris
    device: laptop
    network: wifi-slow
    locale: en-US
  journey: J-operate-remote-gateway-cli
  scenarios: [RT-gateway-remote-cli-profile, RT-gateway-browser-stream-reconnect, RT-gateway-no-device-recovery]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Export and import an identity with correct and wrong passphrases, duplicate profile names, and interrupted writes; existing metadata and credential state must remain atomic."
      - "Run supported and local-only commands through the selected profile, confirming target indication, structured parity, and pre-network policy refusal."
      - "Cut the network during SSE and WebSocket work, restore it, and confirm fresh tickets plus complete or explicitly bounded output."
      - "Revoke the profile identity from another device, distinguish auth from reachability failure, re-pair locally, then remove the old profile and verify metadata plus credential cleanup."
    must_avoid:
      - "Treating SSH forwarding as this profile's transport; reading protected credential bytes as routine evidence."
  evidence_expectations:
    - "Profile lists, protected-store metadata, command target markers, distinct error identities, stream resume boundaries, and atomic removal proof."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->

