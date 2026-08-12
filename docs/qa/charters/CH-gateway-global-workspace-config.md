# CH-gateway-global-workspace-config: Keep global Gateway config authoritative

```yaml
charter:
  id: CH-gateway-global-workspace-config
  mission: "As Dora, use the Feature Tour to start and reconfigure Compozy when the operator home is also a registered workspace, proving the global Gateway configuration stays authoritative without losing workspace registration."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-expose-and-pair-gateway
  scenarios: [MS-gateway-config-ceiling, RT-gateway-local-only-boot]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Start the daemon from the Compozy repository while the operator home remains registered as a workspace and gateway.enabled is set globally."
      - "Read structured daemon and Gateway status, then apply gateway.enabled=true again through the global config command."
      - "Confirm the project workspace and operator-home workspace remain registered after the live write."
      - "Confirm the configuration file remains the operator-global config and no workspace-only Gateway error appears."
    must_avoid:
      - "Creating a second COMPOZY_HOME or removing the operator-home workspace to bypass the collision."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
