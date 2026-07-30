# CH-memory-extractor-hard-cut: Inspect extractor failures without the removed pending alias

```yaml
charter:
  id: CH-memory-extractor-hard-cut
  mission: "As Dora, inspect and operate the memory extractor through CLI, HTTP, and UDS, proving list-failures is the only failure-list verb and that status, replay, and drain remain truthful on empty and stopped states."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-digest-sessions-into-memory
  scenarios: [MS-019]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Run status, list-failures, replay, and drain through the installed CLI and compare the matching HTTP/UDS responses."
      - "Invoke list-pending and confirm Cobra rejects it instead of silently aliasing the new verb."
    must_avoid:
      - "Triggering a provider-backed dream run; this charter owns extractor operator surfaces only."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
