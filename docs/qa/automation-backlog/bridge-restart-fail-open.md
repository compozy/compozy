# Bridge restart fail-open matrix

- Legacy ID: AB-015
- Source: J-recover-mid-turn-restart / NB-031, NB-bridge-restart-recovery / CH-mid-turn-bridge-restart
- Why automate: restart recovery crosses durable delivery state, scope and workspace ownership, provider editability, stale remote anchors, listener shutdown, and metrics. A manual pass is too easy to make provider-specific or to mistake text replay for recovery.
- Suggested layer: serialized runtime E2E matrix over every bridge provider and both editable and append-only fake-provider behaviors.
- Spec sketch: interrupt an active delivery at deterministic phases, restart the daemon, and assert one visible standard terminal error is emitted in the exact scope and workspace before new registration side effects. Assert no persisted answer text is replayed, routes survive, stale anchors fail open, and durable metrics retain their pre-restart values.
- Status: proposed
