# CH-rename-session-parity: Rename one session through every public plane

```yaml
charter:
  id: CH-rename-session-parity
  mission: "As Dora, rename active and archived user sessions and prove durable Web, CLI, HTTP, UDS, and native-tool parity without changing their work."
  mode: strategy-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-rename-session
  scenarios: [RT-session-rename-durable]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Rename from a row and from a session topbar; refresh and compare structured public reads."
      - "Rename an archived session and confirm its archive marker and history remain."
      - "Try blank, overlong, managed, and foreign-workspace requests and confirm no mutation."
    must_avoid:
      - "Do not use database or metadata-file reads as verdict evidence."
```

<!-- Immutable charter: each run's debrief belongs in its dated report. -->
