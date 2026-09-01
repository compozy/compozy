# CH-forced-loop-cancel: Stop a Loop without hunting for its sessions

```yaml
charter:
  id: CH-forced-loop-cancel
  mission: "As Bruno, cancel a live Loop once and trust that the run closes immediately while CompozyOS stops only the sessions that run owns."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-04
  scenarios: [LP-forced-cancel-owned-sessions]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Cancel while a Goal session and a worker task session are active, then fresh-read the run from a second surface before session cleanup finishes."
      - "Keep one borrowed origin session and one foreign-workspace session active; neither may stop."
      - "Restart after one session stop fails, then confirm durable cleanup finishes without reopening the run."
      - "Repeat Cancel, attempt Resume, use Rerun, and search every public surface for the deleted Kill action."
    must_avoid:
      - "Using database rows or internal service calls as proof."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
