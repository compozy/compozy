# CH-loop-template-snapshot-round-trip: Start a templated parent Loop after preview

```yaml
charter:
  id: CH-loop-template-snapshot-round-trip
  mission: "As Ada, preview and start a parent Loop that passes values into two child Loops, then prove the created Run survives a fresh structured read."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-01
  scenarios: [LP-loop-template-snapshot-round-trip]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Validate and dry-run a parent Loop whose two child Loops omit mode and receive templates from both the parent input and a prior agent output."
      - "Confirm dry-run creates no Run, then submit the same inputs for real and capture the returned Run id."
      - "Read that exact Run through a fresh structured surface and confirm the persisted status and definition digest."
      - "Repeat the dry-run once to confirm the boundary is deterministic."
    must_avoid:
      - "Waiting for external agent work to finish; this charter ends when persistence and fresh public readback are proven."
```
