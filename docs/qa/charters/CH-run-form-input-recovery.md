# CH-run-form-input-recovery: Prepare a handoff Run and recover missing input

```yaml
charter:
  id: CH-run-form-input-recovery
  mission: "As Dora, prepare a Studio handoff Run, understand its inherited settings and active-run notice, recover a missing input through Dry run, then start or abandon it without an accidental duplicate."
  mode: scenario-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-05
  scenarios: [LP-web-run-form-section-grammar]
  tour: Back-Button Tour
  time_box_minutes: 45
  guidance:
    must_try:
      - "Use a real Loop with required and optional inputs and a parked concurrent Run; enter its Run form through Web."
      - "Inspect the four collapse sections, their default expansion, input labels and required marks, goal lede, environment gist, limits controls and action-row feedback."
      - "With the required field missing, keep Start run disabled, activate Dry run and require a reachable inline field error with no Run or dry-run request."
      - "Supply valid inputs, change environment and limits, inspect collapsed gists and observe request-pending feedback, then require a public dry-run plan without a new Run."
      - "Leave an edited form and reopen it; compare the persisted configuration and Run roster to prove abandonment did not save overrides or create work."
      - "Start one valid owned Run, compare submitted inputs and effective settings through public readback, then finish or cancel only the Runs owned by this walk."
    must_avoid:
      - "Treating a component test, source inspection or historical screenshot as current UI proof."
      - "Changing another Profile, unrelated Run, stored config or source definition merely to make the form pass."
```

<!-- Immutable charter: each run's debrief belongs in the dated report. -->
