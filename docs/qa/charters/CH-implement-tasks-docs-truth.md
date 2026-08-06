# CH-implement-tasks-docs-truth: Copy the shipped Loop from public docs

```yaml
charter:
  id: CH-implement-tasks-docs-truth
  mission: "As Dora, open the implement-tasks example and prove its name, inputs, graph, commands, and copied YAML match the bundled artifact."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-evaluate-compozy-beta
  scenarios: [ET-site-docs-examples-wave-one]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Reach the example from the Examples index and confirm the removed software-delivery route is absent."
      - "Copy the YAML and compare it with extensions/dev-cycle/loops/implement-tasks/loop.yaml."
      - "Confirm the page documents slug, implementer, and auto_commit only and never claims review, verify, or approve."
      - "Follow the shown CLI command shape against the generated CLI reference."
    must_avoid:
      - "Rechecking the other four wave-one examples beyond one adjacent-link canary."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
