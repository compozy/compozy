# CH-implement-tasks-first-run: Run authored tasks without hidden final gates

```yaml
charter:
  id: CH-implement-tasks-first-run
  mission: "As Bruno, start both implement-tasks modes from the bundled catalog and prove each selected path stops at task implementation and collection."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-01
  scenarios: [TA-080, LP-001, LP-002, LP-003, LP-046]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Find implement-tasks in the Built-in catalog and confirm software-delivery is absent."
      - "Confirm the run form declares slug, implementer, auto_commit, mode, orchestrator, and four optional category runtime inputs."
      - "Inspect the materialized graph and confirm both delivery paths join at collect with no review, verify, or approve nodes."
      - "Run a small authored task graph in each mode and independently read the persisted terminal state and not_taken path."
    must_avoid:
      - "Treating review-and-fix as part of this Loop; it remains a separate bundled Loop."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
