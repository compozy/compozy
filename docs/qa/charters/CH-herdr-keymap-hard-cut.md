# CH-herdr-keymap-hard-cut: Rebind and preset the daemon-owned effective keymap atomically

```yaml
charter:
  id: CH-herdr-keymap-hard-cut
  mission: "As Bruno, edit arrays and numbered ranges, apply and revert the Terminal preset, and prove every accepted or rejected change has one daemon-owned effective result."
  mode: charter-with-tour
  cycle_tier: targeted
  cycle_role: primary
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-administer-window-manager
  scenarios: [ET-layout-editor-shortcut-recorder, MS-configure-window-manager, MS-terminal-shortcut-preset, MS-window-shortcut-arrays-ranges]
  tour: Multi-Tab Tour
  time_box_minutes: 60
  adrs: [ADR-004, ADR-006]
  safety_invariants: [10]
  visual_contract: "docs/design/opendesign/herdr-parity/: task_05 VC-01..VC-06"
  guidance:
    must_try:
      - "Read defaults, overrides, and effective bindings through Settings, config CLI, HTTP, and UDS before changing anything."
      - "Persist alternates, scalar and array ranges, one explicit disable, and a partial range override; name the exact member in blocked and shadowed conflicts."
      - "Race two Settings tabs with a rejected full-map conflict and prove the last known-good keymap remains active everywhere."
      - "Preview, apply twice, and revert the Terminal preset; displaced defaults, platform hazards, idempotence, and exact pre-preset restoration must agree with the daemon."
    must_avoid:
      - "Do not use TypeScript literals or the prototype as a chord authority, and do not accept a partial config write."
```

<!-- Durable targeted charter; every run debrief belongs in its dated report. -->
