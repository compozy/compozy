# CH-scheduled-session-restart-recovery: Recover a scheduled session across restart

```yaml
charter:
  id: CH-scheduled-session-restart-recovery
  mission: "As Bruno, restart the daemon while a recurring agent job is active and prove the replacement becomes ready, the schedule resumes with unique fires, and every public read agrees."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-recover-scheduled-job-restart
  scenarios: [TA-scheduled-session-restart-recovery]
  tour: Interrupt Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Restart immediately after the first scheduled run appears, then reload through the replacement daemon instead of trusting optimistic state."
      - "Wait for another scheduled fire and compare unique fire ids, scheduler registration, and linked sessions through browser, HTTP, UDS, and CLI reads."
      - "Inspect browser console and daemon health after the restart for hidden recovery failures."
    must_avoid:
      - "Do not edit the database, skip the real restart, add fixed sleeps, or treat the restart request itself as proof of recovery."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run report (Session Debriefs), never here. -->
