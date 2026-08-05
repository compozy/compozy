# CH-archive-session-structured-parity: Archive through every agent-manageable surface

```yaml
charter:
  id: CH-archive-session-structured-parity
  mission: "As Ada, archive and restore one stopped session through structured public surfaces, proving exact filters, retained history, deterministic errors, and workspace isolation without using the Web UI."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-archive-session-without-deleting
  scenarios: [RT-session-archive-catalog, RT-011]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Create stopped sessions in two workspaces; archive through CLI, then compare default, archived-only, and inclusive catalog reads over HTTP, UDS, native tools, and extension Host API."
      - "Read status and history while archived, restart the daemon, and confirm the same marker and records before unarchiving."
      - "Attempt archive while active, prompt/resume while archived, repeated archive/unarchive, an invalid archive filter, and a cross-workspace request; every rejection must be deterministic and preserve both catalogs."
      - "Compare generated help and structured output with the documented archive/unarchive contract."
    must_avoid:
      - "Database reads or internal helpers as evidence; every confirmation must come from CLI, HTTP, UDS, native tools, or the published Host API."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report. -->
