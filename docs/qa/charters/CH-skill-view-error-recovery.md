# CH-skill-view-error-recovery: Give an agent an actionable skill-read failure

```yaml
charter:
  id: CH-skill-view-error-recovery
  mission: "As Ada, distinguish and recover from missing-resource and malformed-definition failures through the native skill seam without gaining operator authority."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-load-skill-in-managed-session
  scenarios: [ET-skill-view-actionable-errors]
  tour: Garbage Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Call skill_view for a missing relative resource and for a named skill with invalid YAML, then compare native, hosted MCP, and public HTTP reason codes and safe messages."
      - "Confirm the separately marked operator details name the malformed file and YAML location while the primary public message remains stable and safe."
      - "Repair the definition and repeat the native read without restarting the daemon; confirm no managed CLI or direct file-read fallback occurred."
    must_avoid:
      - "Do not accept a generic backend failure or retry an unchanged permanent error until it happens to pass."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
