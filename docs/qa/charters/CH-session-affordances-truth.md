# CH-session-affordances-truth: Titles, salvaged intent, and honest mutation markers

```yaml
charter:
  id: CH-session-affordances-truth
  mission: "As Théo, verify the small session affordances tell the truth: one generated title per unnamed session, interrupt→steer salvages the cancelled intent exactly once, failed edits carry the verifier marker, and the session CWD survives resume."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-11
  scenarios: [RT-session-lifecycle-affordances, RT-session-cwd-resume]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Unnamed session: exactly one title spawn after the first assistant response, none after the second; explicit names never overwritten; title lands in HTTP/UDS/CLI/Web catalogs."
      - "Interrupt then steer → the composed cancelled+correction input enqueued once under the new generation; interrupt then plain new prompt → no salvage composition."
      - "Failed edit with no later success → verifier marker in the durable timeline; a later successful edit for the same path suppresses it (agh-ui-screenshot both states)."
      - "Nested-CWD session: provider observes the mapped runtime path, resume returns to the same directory, outside-workspace CWD stays rejected."
    must_avoid:
      - "Testing compaction here — CH-crash-resume-compaction owns that half of J-11."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
