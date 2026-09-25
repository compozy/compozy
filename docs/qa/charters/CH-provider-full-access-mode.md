# CH-provider-full-access-mode: Verify trusted provider commands stay within session policy

```yaml
charter:
  id: CH-provider-full-access-mode
  mission: "As Ada, enable the operator's provider full-access preference and prove Codex and Claude Code sessions can reach owned local services while narrower session policy remains authoritative."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-15
  scenarios: [RT-provider-full-access-mode]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Inspect resolved Codex and Claude Code modes from the CLI in a disposable workspace."
      - "Prompt each provider to read only an owned Docker container and local PostgreSQL fixture."
      - "Check that an explicit narrower mode wins and a restricted session cannot apply an unrestricted mode."
    must_avoid:
      - "Do not use services, workspaces, or provider processes from another QA run."
```

<!-- The charter is durable and immutable: debriefs belong in dated run reports. -->
