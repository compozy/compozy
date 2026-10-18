# CH-automation-catalog-recovery: Keep automation catalogs truthful through interruptions

```yaml
charter:
  id: CH-automation-catalog-recovery
  mission: "As Bruno, run editorial automations and recover from slow requests, unavailable connectivity and a paused automation runtime while keeping saved catalogs, details and creation context trustworthy."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-24
  scenarios: [ET-web-jobs-triggers-catalog]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Enter Jobs through the Dock, run a global job whose configured agent is unavailable, and read its durable failure after reload without losing the detail URL or Global lens."
      - "Start two different Loop jobs while the connection is slow; each keeps its own pending control and a repeated activation cannot create duplicate runs. Confirm the resulting histories independently."
      - "Lose browser connectivity after loading catalog rows and a detail. Try refresh and continuation; retain useful saved data while clearly showing failure, then restore connectivity."
      - "Open and abandon Loop-seeded Add schedule/Add trigger editors, then seed another Loop in the same catalog. The seed is consumed once, unfiltered catalogs remain available, and route/workspace changes clear old editor or queued-run state."
      - "Pause automation through the supported config writer and restart the owned daemon while the browser retains cached Jobs. Inspect rows, cards and detail: every Run now action must be unavailable and public dispatch must refuse the disabled runtime. Restore the original setting and verify recovery."
    must_avoid:
      - "Source/storage inspection in persona, mocked API/agent responses, query-cache injection, or accepting optimistic UI as persisted evidence."
      - "Changing another lab or leaving automation runtime disabled after the session."
```
