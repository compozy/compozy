# CH-implement-tasks-dry-run: Preview the reduced task implementation graph

```yaml
charter:
  id: CH-implement-tasks-dry-run
  mission: "As Lea, dry-run implement-tasks and confirm the reduced inputs and five-node plan are truthful and side-effect free."
  mode: charter-with-tour
  persona:
    name: Lea
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-02
  scenarios: [LP-006, LP-007]
  tour: Garbage Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Dry-run with a valid slug and confirm the plan contains slug_input, load_tasks, implement, execute_task, and collect only."
      - "Read the Runs list through a fresh public request and confirm no run was created."
      - "Submit with the required slug absent and confirm no plan or run appears."
      - "Repeat the dry-run and confirm it remains side-effect free."
    must_avoid:
      - "Starting a real run; CH-implement-tasks-first-run owns that action."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
