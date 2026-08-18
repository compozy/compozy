# CH-loop-partial-runtime: Settle wide partial work without duplicate lanes

```yaml
charter:
  id: CH-loop-partial-runtime
  mission: "As Bruno, interrupt routed wide fan-outs under fail_fast and best_effort and prove monotonic settlement, bounded width, exact lane control, and truthful partial completion."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-complete-partial-loop
  scenarios: [LP-best-effort-partial, LP-fail-fast-lane-cancel, LP-exclusive-route-history, LP-wide-fanout-window, LP-broken-stop-when-policy, LP-fanout-progress-naming, LP-per-lane-node-control]
  tour: Interrupt Tour
  time_box_minutes: 60
  invariants: [Safety 4 monotonic settlement, Safety 5 post-commit lane cancel, Safety 6 exactly-once windowing, Safety 10 deterministic route, Safety 11 per-lane gate revisions]
  guidance:
    must_try:
      - "Publish routing, predicate-policy, naming, and strategy variants from the documented grammar."
      - "Restart during a 500-lane window and verify active width, stable progress, and no duplicate cell."
      - "Address one fan-out item through respond, amend, pause, resume, cancel, kill, and rerun surfaces without changing siblings."
      - "Compare collect output, completion_state, route causes, branch_pruned events, and SSE terminal payloads after refresh."
    must_avoid:
      - "Treating strategy cancellation as failure or a partial run as complete."
```

