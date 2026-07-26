# CH-suggestions-consent: A fresh workspace offers automations I control completely

```yaml
charter:
  id: CH-suggestions-consent
  mission: "As Bruno, take a fresh workspace from seeded suggestions to one real firing job and one permanently dismissed card, proving the cap, the CAS, the dedup latch, the lifecycle guard, and workspace isolation on every surface."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-24
  scenarios: [TA-automation-suggestions]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Fresh workspace first list seeds 3–5 workspace-scoped pending suggestions; accept one via the Web card and another via CLI/native — each creates a real Job through normal validation that fires under the scheduler."
      - "Dismiss one and prove the latch: the identical suggestion never re-emits across reload; race two resolutions of one suggestion → exactly one wins, the loser gets ErrSuggestionResolved."
      - "Pending cap enforced at insert; a lifecycle-command prefill is rejected by the creation-seam guard with nothing persisted."
      - "Workspace B sees none of workspace A's suggestions on list/accept/dismiss."
    must_avoid:
      - "Hand-authoring jobs to shortcut the flow — the point is zero hand-authoring (A1)."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
