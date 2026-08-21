# CH-run-agent-session-lifecycle: Complete a Loop without leaking worker sessions

```yaml
charter:
  id: CH-run-agent-session-lifecycle
  mission: "As Bruno, start a nested Loop from Batuta, follow one run-agent worker through completion, and trust both its family lineage and terminal session state."
  mode: strategy-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-complete-partial-loop
  scenarios: [LP-run-agent-session-lifecycle]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Start the Loop from a real Batuta session and capture the child Loop Run plus its bound run-agent session through structured output."
      - "Compare parent_session_id and root_session_id through CLI and one independent HTTP or UDS read while the worker is active."
      - "Let the worker return schema-valid output, refresh the Loop and session reads, and confirm the worker is stopped while Batuta remains active."
      - "Exercise one retry-eligible failure and confirm the same binding stays active until the cell settles terminally."
    must_avoid:
      - "Using database reads, unit tests, or source inspection as pass evidence."
      - "Stopping the worker or Batuta manually before the terminal-state check."
```

<!-- The charter is durable and immutable: each run's debrief belongs in the dated report. -->
