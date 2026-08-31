# CH-forced-cancel-settlement: Leave no live work after Cancel

```yaml
charter:
  id: CH-forced-cancel-settlement
  mission: "As Ada, cancel a live Loop and prove that no task remains claimable before and after daemon restart."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-loop-terminal-recovery
  scenarios: [LP-terminal-loop-settlement]
  tour: Interrupt Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Cancel with coordinator and worker task runs active, then list and inspect those tasks through public CLI reads."
      - "Restart the daemon immediately after the terminal response and repeat the same reads."
      - "Try to claim work owned by the canceled run and inspect its task timeline reason."
      - "Repeat Cancel and confirm no duplicate settlement or cleanup effect appears."
    must_avoid:
      - "Seeding or reading private database state during the session."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
