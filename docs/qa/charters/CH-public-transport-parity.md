# CH-public-transport-parity: Read the same work over either daemon transport

```yaml
charter:
  id: CH-public-transport-parity
  mission: "As Ada, use the documented CLI, HTTP, and UDS interfaces to read and control the same work without changing its ownership, validation, or pagination semantics."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-15
  scenarios: [RT-042]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Compare workspace, session, provider, agent, settings, and task resources through HTTP and UDS, including their supported CLI reads."
      - "Reuse returned pagination cursors, preserve workspace and profile filters, and compare malformed input, missing resources, and known foreign-workspace or foreign-profile refusals."
      - "Stop an owned session through a public surface, then compare its stored detail, status, and history with its catalog state; confirm the reads remain stable after a fresh request."
      - "Record transport-specific loopback and privileged-mutation boundaries separately from domain payload equality."
    must_avoid:
      - "Do not infer every domain's native-tool or Web behavior from transport parity; their scenarios own those walks."
      - "Do not use source inspection or direct database reads as an independent product read path."
```

<!-- Immutable mission; execution debriefs belong in the dated report. -->
