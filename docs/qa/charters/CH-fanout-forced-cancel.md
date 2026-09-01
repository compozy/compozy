# CH-fanout-forced-cancel: Cancel one fan-out lane and preserve its siblings

```yaml
charter:
  id: CH-fanout-forced-cancel
  mission: "As Bruno, cancel one active fan-out lane by its exact item identity while every sibling keeps its own state and session."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-complete-partial-loop
  scenarios: [LP-per-lane-node-control]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Address one active lane through Web, CLI, HTTP, and native item_index forms."
      - "Fresh-read at least one healthy sibling and prove its state and session did not change."
      - "Repeat the addressed Cancel and try a stale or unknown lane identity."
      - "Confirm the per-lane Kill route, command, and native tool are absent."
    must_avoid:
      - "Using strategy-driven sibling cancellation as a substitute for operator Cancel."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
