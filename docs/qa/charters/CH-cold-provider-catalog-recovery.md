# CH-cold-provider-catalog-recovery: Recover a missing provider catalog on first open

```yaml
charter:
  id: CH-cold-provider-catalog-recovery
  mission: "As Sol, open Cursor in the first runtime selector of a fresh install and get usable model rows without manually reloading the catalog."
  mode: scenario-based
  persona:
    name: Sol
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-17
  scenarios: [RT-model-catalog-cold-open]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Start with no persisted Cursor rows, open the first selector once, choose Cursor, and wait only for visible product feedback."
      - "Confirm one automatic refresh produces logical Cursor rows and valid option controls without pressing Reload."
      - "Close and reopen the selector, then refresh the page; confirm rows remain usable and no duplicate refresh loop appears."
      - "Use keyboard navigation and confirm the loading state never traps focus or presents an empty result as final."
    must_avoid:
      - "Do not warm the catalog through CLI, HTTP, or a prior selector open."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
