# CH-forced-node-cancel-ui: Cancel live work from the run page

```yaml
charter:
  id: CH-forced-node-cancel-ui
  mission: "As Bruno, cancel live run and node work from the run page and trust every control, transition, and terminal story after refresh."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-recover-loop-node-failure
  scenarios: [LP-operator-lifecycle-ui, LP-web-run-page-section-grammar, LP-live-pause-repair-resume]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Cancel running, paused, waiting, and quarantined node states through only the actions the payload allows."
      - "Cancel a run from the destructive header action and confirm one calm Run canceled terminal beat after refresh."
      - "Confirm no Kill action, dialog, event beat, or pending state appears anywhere in the run page or inventories."
      - "Compare the fresh page with structured CLI and HTTP reads for the same run and node."
    must_avoid:
      - "Treating optimistic UI state or a component fixture as the final observable."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
