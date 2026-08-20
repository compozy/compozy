# CH-loop-graph-runtime-safety: Filter lanes and preserve owned structured output

```yaml
charter:
  id: CH-loop-graph-runtime-safety
  mission: "As Bruno, run a filtered fan-out whose agents publish typed large results and trust that only matching lanes exist, exact valid output reaches the next node, and the daemon alone settles worker tasks."
  mode: strategy-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-complete-partial-loop
  scenarios: [LP-fan-out-filtering, LP-run-agent-output-ownership]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Validate and run a bind_as/index_as filter that removes enough raw elements to satisfy max_fan_out only after filtering, then compare lane order and batches through status."
      - "Exercise a valid zero-match filter and a predicate error governed by on_eval_error without creating excluded worker lanes."
      - "Publish schema-valid output above the inline threshold, read its exact fields from the next node, and compare the terminal output after a fresh status read."
      - "From the bound worker session, send a heartbeat and attempt complete and fail; heartbeat must remain available while terminal settlement stays daemon-owned."
    must_avoid:
      - "Using internal database reads as pass evidence or treating a test helper as a public runtime surface."
      - "Prompting the worker to describe the QA activity or its expected verdict."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
