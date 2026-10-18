# CH-session-derive-interrupt: Retries, restarts, provider failures, and upgrades never duplicate or lose a session

```yaml
charter:
  id: CH-session-derive-interrupt
  mission: "As Théo, interrupt derive and prompt flows (repeat requests, delete children, restart the daemon mid-flow, rate-limit a turn, boot a pre-feature home) and prove every outcome is recorded once, offered truthfully, and upgraded without loss."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-15-operate-session-via-cli-api
  scenarios: [RT-session-derive-retry, RT-provider-error-handoff, RT-session-lineage-upgrade, RT-conversation-rewind]
  tour: Interrupt Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Same idempotency key after the source moved on: `Replayed  yes`, recorded counts; after deleting the child: `child_deleted: true`, nothing created; same key with another agent: `idempotency_conflict`."
      - "Stop the daemon right after a continue with `--message` returns, restart, repeat the command: the message runs exactly once."
      - "Rate-limit a user session's turn (acpmock prompt-error step): `next_action: handoff` with guidance naming `compozy session continue`; nothing is created until the operator continues; a spawned child keeps `retry`."
      - "Boot the branch build on a home written by the pre-feature build: lineage kinds are truthful, the --parent child rewinds, the spawned child is refused, the pre-feature root continues and forks."
    must_avoid:
      - "Editing SQLite or meta.json by hand to manufacture a state — only public surfaces and the pre-feature binary create data."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
