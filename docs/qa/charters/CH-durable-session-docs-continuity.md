# CH-durable-session-docs-continuity: Read one consistent stopped-session lifecycle

```yaml
charter:
  id: CH-durable-session-docs-continuity
  mission: "As Dora, follow the migration and session guides and prove they consistently teach how a stopped session continues without confusing prompt restart with live attachment."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-evaluate-compozy-beta
  scenarios: [ET-site-docs-first-session]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Enter through the migration guide, then follow the Web UI, session lifecycle, resume, and generated runtime-command pages in the same order a beta evaluator would discover them."
      - "Confirm every page distinguishes a normal prompt, which restarts a stopped agent process and keeps the session history, from session resume, which only attaches another client while that process is live."
      - "Reload the Web UI and lifecycle pages directly and confirm the runtime set and clear references remain reachable without contradictory terminal-stop guidance."
    must_avoid:
      - "Do not infer the docs from source files during the session; use only the rendered site and its public navigation."
      - "Do not repeat the real-provider runtime walk; the stopped-session and runtime-continuity charters own that behavior."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report, never here. -->
