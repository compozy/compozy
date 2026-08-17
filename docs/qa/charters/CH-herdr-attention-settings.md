# CH-herdr-attention-settings: Keep attention policy identical across every settings surface

```yaml
charter:
  id: CH-herdr-attention-settings
  mission: "As Dora, change every attention setting through a different public surface and prove the live daemon, persisted file, Web state, and delivery behavior never diverge."
  mode: charter-with-tour
  cycle_tier: targeted
  cycle_role: primary
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-administer-runtime-settings
  scenarios: [MS-attention-settings-roundtrip]
  tour: Multi-Tab Tour
  time_box_minutes: 60
  adrs: [ADR-002]
  safety_invariants: []
  visual_contract: "docs/design/opendesign/herdr-parity/: task_03 VC-21..VC-23"
  guidance:
    must_try:
      - "Write toasts, sound, system, and muted_workspaces through Web, config CLI, HTTP, and UDS in sequence; fresh-read each value from every other surface."
      - "Keep Settings open in two tabs, race complete-section writes, reload both, and prove a whole valid config wins without field loss."
      - "Mute two workspaces, delete one, and confirm pruning removes only that id while the active runtime keeps the other policy."
      - "Exercise Armed, Denied, and Unavailable system-channel states against real browser capability and permission."
    must_avoid:
      - "Do not parallelize config writes against the isolated home or infer OS delivery from a saved boolean."
```

<!-- Durable targeted charter; every run debrief belongs in its dated report. -->
