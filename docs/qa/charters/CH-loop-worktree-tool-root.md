# CH-loop-worktree-tool-root: Loop tools use the selected checkout

```yaml
charter:
  id: CH-loop-worktree-tool-root
  mission: "As Ada, run a Loop whose task pack exists only in a ready Worktree and prove its extension action uses that checkout."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-isolated-task-loop-execution
  scenarios: [LP-loop-environment-resolution]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Create the task pack only in a ready Worktree, configure a Loop extension action with that worktree id, and start it through structured CLI output."
      - "Read the Loop output through CLI, API, and Web; it must name the task path under the selected Worktree."
      - "Use a root-scoped run or direct extension action as a canary; it must not see the worktree-only task pack."
    must_avoid:
      - "Using database reads, copied files in the parent checkout, or test-only fixtures as behavior evidence."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
