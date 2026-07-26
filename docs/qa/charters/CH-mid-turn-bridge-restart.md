# CH-mid-turn-bridge-restart: Restart mid-turn and recover with a visible terminal

```yaml
charter:
  id: CH-mid-turn-bridge-restart
  mission: "As Omar, interrupt a materially acknowledged bridge turn at daemon restart and prove checkpoint-only boot recovery posts one visible terminal error, replays no text, preserves metrics, and fences new registrations until reconciliation completes."
  mode: charter-with-tour
  persona:
    name: Omar
    device: desktop
    network: flaky
    locale: en-US
  journey: J-recover-mid-turn-restart
  scenarios: [NB-031, NB-bridge-restart-recovery]
  tour: Interrupt Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Start a fake-provider turn, capture a material remote ACK, then terminate the daemon between streamed updates and terminal delivery while preserving the isolated AGH_HOME/GlobalDB."
      - "On boot, race a new registration against reconciliation; it must remain inadmissible until every active row has produced its one standard visible terminal error."
      - "Use edit-capable and append-only/stale-anchor fixtures; no prior text or acknowledged prefix may be replayed, and the recovery must not depend on the old remote message still existing."
      - "Read delivery metrics before/after restart, then submit one fresh turn and confirm normal delivery in the same route."
    must_avoid:
      - "Expecting content resume as the normal contract; deleting the lab database between restarts; treating a daemon log as sufficient channel evidence."
  evidence_expectations:
    - "Pre-kill ACK and ledger identifiers, boot/reconciliation/admission timestamps, channel/provider request log, durable metrics before/after, and the successful post-recovery turn."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
