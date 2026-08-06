# CH-implement-tasks-first-run: Run authored tasks without hidden final gates

```yaml
charter:
  id: CH-implement-tasks-first-run
  mission: "As Bruno, start implement-tasks from the bundled catalog and prove its public contract and runtime stop at task implementation and collection."
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
      - "Confirm the run form declares only slug, implementer, and auto_commit."
      - "Inspect the materialized graph and confirm it ends at collect with no review, verify, or approve nodes."
      - "Run a small authored task graph and independently read the persisted terminal state."
    must_avoid:
      - "Treating review-and-fix as part of this Loop; it remains a separate bundled Loop."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
