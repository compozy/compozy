# CH-operator-lifecycle-ui: Operate one Loop lane without losing daemon truth

```yaml
charter:
  id: CH-operator-lifecycle-ui
  mission: "As Bruno, inspect a real Loop lane as it retries, pause and resume it from the run page, and use the workspace inventories to find parked work without seeing a control or count the daemon did not publish."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-04
  scenarios: [LP-operator-lifecycle-ui]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Open a real retrying run from Runs; confirm attempt and next-attempt time are visible, then refresh and reopen the deep link to prove the state is durable."
      - "Pause the retrying lane in drain mode from its row menu, read the recorded provenance, and confirm the fresh run detail and waiting/retrying inventories agree with the page."
      - "Resume the same lane using the offered mode and confirm the fresh read no longer reports it paused; do not accept optimistic UI as proof."
      - "Open each waiting, quarantined, attention, and retrying inventory; verify truthful empty states, filter controls, real state-age sorting, and Load more only when a cursor exists."
      - "Inspect a real quarantine entry when available and verify hint, attempt chain, target, input reference, and requeue confirmation; if the lab cannot create that state through a public surface, record the exact parity gap rather than substituting Storybook."
      - "Try refresh, browser back/forward, one rapid double-click on a safe control, keyboard-only navigation, 200% browser zoom, and a 768px viewport."
    must_avoid:
      - "Using Storybook, fixtures, SQLite, source reads, or internal endpoints as behavior proof."
      - "Acting on a node after its payload no longer declares the verb; a stale or rejected action is a finding."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
