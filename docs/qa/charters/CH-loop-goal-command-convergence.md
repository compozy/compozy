# CH-loop-goal-command-convergence: Finish a command-judged Goal without repeated work

```yaml
charter:
  id: CH-loop-goal-command-convergence
  mission: "As Bruno, run a Goal whose deterministic judge checks the workspace result, then understand any rejection and watch the Run advance as soon as the check passes."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-26
  scenarios: [LP-goal-command-judge]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Start from the documented Loop CLI, observe one command-judged Goal, and read its settled turn through a fresh public status surface."
      - "Prove the command runs from the selected workspace and that exit 0 advances to the declared successor without an approval pause."
      - "Read the durable criterion outcome, exit code, output, blockers, and warnings through structured status and the Web timeline."
    must_avoid:
      - "Do not resume or mutate the historical operator Run."
      - "Do not query SQLite or use an internal handler as pass evidence."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
