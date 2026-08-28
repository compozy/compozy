# CH-provider-runtime-strategies: Apply runtime controls through the owning strategy

```yaml
charter:
  id: CH-provider-runtime-strategies
  mission: "As Théo, use the Runtime Selector and public prompt surfaces to prove standard ACP, Cursor launch-bound, and OpenClaw provider-managed controls behave truthfully."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-17
  scenarios: [RT-cursor-logical-runtime-options, RT-openclaw-provider-managed-runtime, RT-session-prompt-runtime-transitions, RT-session-runtime-selection-continuity, ET-web-runtime-selector-minimal-slider, RT-070, RT-061]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Run valid Grok 4.5 and 4.6 Reasoning/Fast combinations plus one Opus 5 Thinking combination, then change a launch-bound value and prove atomic process replacement."
      - "Persist speed and typed options as agent defaults, durable session selection, and prompt overrides; stop and restart before proving the precedence and public logical state."
      - "Select OpenClaw and confirm Provider managed appears, unavailable controls stay disabled, no-model binding works, and an explicit unsupported override fails."
      - "On a standard ACP, exercise one grouped select and one boolean update and verify model-first ordering plus response-by-response revalidation."
    must_avoid:
      - "Do not use private Cursor alias text as the public model ID or infer support from a model name."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report. -->
