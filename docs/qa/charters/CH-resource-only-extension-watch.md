# CH-resource-only-extension-watch: Rebuild a passive extension from a live source edit

```yaml
charter:
  id: CH-resource-only-extension-watch
  mission: "As Bruno, build a passive extension with an agent, a skill, and a Network requirement, keep dev watch running while editing the skill, and confirm the new generation and workspace resources through public surfaces."
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
      - "Build without package.json or go.mod, then read the generated manifest and confirm its Network requirement matches the source."
      - "Start extension dev with --watch, edit SKILL.md while the process remains active, and capture the reload output with a different generation hash."
      - "Read both the agent and skill from the selected workspace and confirm neither appears in the global catalogs."
      - "Run the existing code-backed authoring lifecycle as the adjacent compatibility canary."
    must_avoid:
      - "Adding a language toolchain, polling internal storage, or treating the generated file alone as proof that the daemon reloaded it."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in the dated report. -->
