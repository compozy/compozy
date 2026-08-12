# CH-loop-cancel-restart: Recover a canceled Loop coordinator on restart

```yaml
charter:
  id: CH-loop-cancel-restart
  mission: "As Bruno, cancel interrupted Loop work and restart the daemon until I can trust that damaged coordinator state cannot lock me out of every workspace."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-recover-loop-node-failure
  scenarios: [LP-cancel-restart-recovers]
  tour: Interrupt Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Remove each bound session before cancellation, then cancel the deterministic coordinator task while the Loop remains nonterminal."
      - "Restart the daemon twice and use a fresh workspace read after each start to prove readiness rather than trusting the start command alone."
      - "Repeat recovery after canceling the first recovered coordinator run and confirm exactly one new coordinator becomes available."
      - "Check a foreign workspace read so repaired work never crosses its workspace boundary."
    must_avoid:
      - "Reading SQLite as verdict evidence or repairing stored rows outside public CLI and HTTP surfaces."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
