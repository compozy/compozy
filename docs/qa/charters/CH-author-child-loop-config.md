# CH-author-child-loop-config: Can an author scope configuration to one child Loop?

```yaml
charter:
  id: CH-author-child-loop-config
  mission: "As Bruno, author typed finite budgets and runtime rules for one child Loop in the Web editor, publish them, and prove a fresh public read preserves the child-only contract."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-recover-loop-node-failure
  scenarios: [LP-child-loop-config-web-authoring]
  tour: Feature Tour
  time_box_minutes: 30
  surfaces: [Web, HTTP]
  browser_plan: "Drive the editor through agent-browser and verify the saved definition after reload plus a fresh public HTTP read."
  guidance:
    must_try:
      - "Open a real workspace Loop, select its run-loop node, and paste a JSON object containing iteration_cap, budget_tokens, and a runtime_rules array."
      - "Publish once, reload the editor, and compare the same typed object with the public definition response."
      - "Try malformed JSON without publishing, then restore the valid object and confirm no unrelated field changes."
      - "Omit config_overrides in an adjacent parent and confirm its existing authoring path still publishes."
    must_avoid:
      - "Source inspection, mocked HTTP responses, database reads, or direct fixture mutation as verdict evidence."
      - "Mobile editor assertions; the visual Loop editor is desktop-only."
```

<!-- The charter is durable and immutable; run debriefs belong in dated reports. -->
