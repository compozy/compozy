# CH-runtime-ui-regression-agent-skills: Launch a hot agent with one quarantined skill

```yaml
charter:
  id: CH-runtime-ui-regression-agent-skills
  mission: "As Dora, create an agent while the app is open, launch it, then prove one invalid local skill cannot take the agent down."
  mode: scenario-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-17
  scenarios: [RT-agent-hot-discovery-skill-isolation]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Create the agent through the CLI while the browser remains open and confirm it appears without reload."
      - "Launch a session through the web UI, then confirm its durable owner and agent through the CLI or API."
      - "Add one valid and one invalid agent-local skill, create another session, and prove only the invalid skill is absent while a diagnostic is recorded."
    must_avoid:
      - "Do not restart the app to make the agent visible or accept any 4xx/5xx session-create response as isolation."
      - "Do not modify the operator's real Compozy home."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
