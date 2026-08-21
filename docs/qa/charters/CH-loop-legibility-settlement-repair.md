# CH-loop-legibility-settlement-repair: Prove no ended Loop run leaves claimable work behind

```yaml
charter:
  id: CH-loop-legibility-settlement-repair
  mission: "As Ada, end Loop runs every way they can end — completion, cancel, kill, crash, retention prune — and prove that settlement is part of the transition, that the boot barrier fails closed, that the sweep is idempotent, and that every repaired record explains itself through a structured reason."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-loop-terminal-recovery
  scenarios: [LP-terminal-loop-settlement, LP-loop-lifecycle-config-cli]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Kill the daemon mid-run, then boot: neutralization must complete before recovery, the coordinator backstop, the reconciler ticker and any claimer start — attempt a claim immediately after readiness and confirm no work owned by an ended run comes back."
      - "Make neutralization fail at boot: the daemon must NOT report readiness and must start none of those components — a structured startup error, not a log-and-continue (the log-and-retry-next-tick policy is for interval sweeps only)."
      - "Boot a second time on the same seeded state and confirm idempotence — zero duplicate audit events, active runs untouched — then confirm the three reasons are distinct and queryable: loop_run_terminal inline, reconciled_run_terminal from the sweep, run_missing for a retention orphan."
      - "Cancel and kill runs whose children already settled, reading every affected record back through compozy task timeline <task-id>: killed work is never recorded completed and an already-terminal child keeps its own outcome. In the same pass set loops.reconcile_interval to a non-positive duration (named validation message, prior value unchanged), then confirm the boot sweep still runs once whatever the interval says and a changed interval only affects later sweeps."
    must_avoid:
      - "Reading SQLite directly as proof — every verdict comes from a public structured read."
      - "Treating a passing single boot as idempotence; the second boot is the assertion."
      - "Leaving lab processes alive: cite teardown.json with clean true on every terminal path."
```

## Selection rationale

Targeted tier, first by risk. This charter owns Safety Invariants 1-6 and 11 and ADR-006: one
settlement authority inside the transition, a boot barrier that **fails closed** (peer review round 2
B-005), an idempotent status-guarded sweep, no lifecycle mutation on non-terminal runs, serialized
writers, structured distinct reasons, and a sweep that observes but never claims (L-005). It runs
first because clean settlement is what makes every other charter's reads trustworthy — the two
orphaned coordinators from the 2026-08-19 crash are the incident shape this cycle must prove gone.
The Interrupt Tour is the matching lens: the failure mode is a crash between the transition and the
record, so the session's job is to interrupt at every seam.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
