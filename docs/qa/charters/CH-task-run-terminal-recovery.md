# CH-task-run-terminal-recovery: Does one terminal outcome survive races and restart?

```yaml
charter:
  id: CH-task-run-terminal-recovery
  mission: "As Bruno, finish active non-leased task runs through public surfaces and prove that one outcome wins, the session stops first, and restart resumes that outcome exactly once."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-finish-task-run
  scenarios: [TA-026, TA-027, TA-028, TA-029]
  tour: Interrupt Tour
  time_box_minutes: 75
  guidance:
    must_try:
      - "Attach an active session to a non-leased run, then walk complete, fail, and cancel on fresh runs through HTTP and UDS-backed CLI; use the web run header for the cancel branch when available."
      - "Race each chosen action with a different terminal action; the loser receives 409, cannot replace the first outcome, and an independent read shows no duplicate terminal event."
      - "Interrupt the daemon after admission or stop request, restart the same isolated home, and verify the recorded action resumes before ordinary recovery."
      - "Read run, task, session, and event history after settlement; the session is stopped and all durable surfaces agree on one outcome."
    must_avoid:
      - "Editing SQLite directly, using an internal manager method, exposing a claim token, or treating the first command response as proof without an independent read."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report, never here. -->
