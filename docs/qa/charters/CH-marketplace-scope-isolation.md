# CH-marketplace-scope-isolation: Keep MCP install secrets in one scope

```yaml
charter:
  id: CH-marketplace-scope-isolation
  mission: "As Bruno, start an MCP install in project scope, switch to Global, and prove the Global dialog starts clean and submits only the Global destination."
  mode: strategy-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-mcp-authorize-repair
  scenarios: [ET-web-marketplace-mcp-authorize-installed]
  tour: Multi-Tab Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Enter a credential in a project-scoped install, switch Global before submitting, and confirm the new dialog contains no prior secret."
      - "Complete or abandon the Global install and refresh the installed list to confirm its effective scope."
      - "Open the same marketplace detail in a second tab and confirm neither tab can submit the other's destination state."
    must_avoid:
      - "Do not complete a real OAuth consent leg; record that leg for human verification."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
