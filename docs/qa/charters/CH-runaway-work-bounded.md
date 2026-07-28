# CH-runaway-work-bounded: Abuse the kernel until only budgets and breakers are left standing

```yaml
charter:
  id: CH-runaway-work-bounded
  mission: "As Ada, inject crash loops, wedged actions, contested exact claims, and a permanently failing loop node, and prove every runaway path terminalizes with its forensic reason while healthy long work is never harmed."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-bound-runaway-work
  scenarios: [TA-lease-recovery-attempt-budget, TA-action-run-liveness, TA-loop-failure-breaker, TA-exact-claim-single-owner]
  tour: Garbage Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Claim → kill worker → expire, repeatedly: each recovery must consume the durable budget and the row must terminalize needs_attention with lease_recovery_exhausted at max_attempts."
      - "Two-node loop with one always-failing node in both terminal orders: Stalled at the per-node streak, sibling success never resets it; unbounded failing watch hits the hard backstop; healthy loop control never trips."
      - "Race two exact claims on one queued RunID: exactly one owner, typed no-claimable-run for the loser; typed errors on claimed/running targets."
      - "Wedge one heartbeating action past its window and run one healthy long tool at the same time: node_timeout/no_progress for the wedge (budget consumed, loop advances), untouched survival for the healthy run."
    must_avoid:
      - "Weakening timeouts or budgets in config to force outcomes — use the shipped defaults plus documented keys only."
      - "Reading in-memory state as proof; terminal reasons must come from durable run listings."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
