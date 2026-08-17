# CH-resource-only-extension-dev: Iterate on a passive extension kit and preserve last-good resources

```yaml
charter:
  id: CH-resource-only-extension-dev
  mission: "As Bruno, build and dev-link a passive extension kit with no language project, then edit, reload, recover from an invalid resource, and confirm its declarations stay isolated to the selected workspace."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-dev-lifecycle
  scenarios: [ET-resource-only-extension-dev, ET-extension-dev-reload-loop]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Build the real Batuta source twice and compare generation hashes before validating the copied agent, loop, and skill resources."
      - "Dev-link a minimal passive agent kit, read the agent through the workspace API, edit and reload it, then independently read the changed prompt."
      - "Break the agent definition, confirm reload fails and the prior generation remains readable, then remove the dev link and verify the workspace resource disappears."
      - "Run the existing code-backed authoring lifecycle as an adjacent compatibility canary."
    must_avoid:
      - "Adding package.json or go.mod, installing the extension globally, reading the registry database, or using an internal-only API as proof."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in the dated report (Session Debriefs), never here. -->
