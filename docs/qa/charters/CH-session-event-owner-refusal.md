# CH-session-event-owner-refusal: Refuse a foreign session event store without rewriting it

```yaml
charter:
  id: CH-session-event-owner-refusal
  mission: "As Bruno, recover from a session directory containing another workspace's event store without letting Compozy relabel, migrate, or mutate the foreign database."
  mode: strategy-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-daemon-schema
  scenarios: [RT-session-event-owner-isolation]
  tour: Garbage Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Create two real sessions in different workspaces, stop the isolated daemon, preserve both complete directories, and place the second session's intact events.db family under the first session directory."
      - "Record every target database and sidecar digest, then require CLI, HTTP, and UDS history/event reads to refuse the first session without changing any digest."
      - "Confirm the untouched source session still reads normally, restore the first session's matching complete directory, restart, and confirm its history is readable again."
    must_avoid:
      - "Editing session_db_owner, Goose version rows, or event rows; using compozy session repair; moving one live database file while the daemon is running."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in its dated report (Session Debriefs), never here. -->
