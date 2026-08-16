# CH-herdr-task-approval-canary: Preserve task approvals beside session attention

```yaml
charter:
  id: CH-herdr-task-approval-canary
  mission: "As Cora, find and decide one approval-gated task from the existing attention surfaces and prove the new session sections did not hide, double-count, or misroute it."
  mode: charter-with-tour
  cycle_tier: targeted
  cycle_role: adjacent-canary
  canary_rationale: "The Herdr parity bell rewrote the shared attention model while task approvals remain an existing consumer of the same row, count, and landing path."
  persona:
    name: Cora
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-operate-home-dashboard
  scenarios: [RT-home-approve-from-dashboard]
  tour: Feature Tour
  time_box_minutes: 30
  adrs: [ADR-002]
  safety_invariants: []
  guidance:
    must_try:
      - "Seed one pending task approval beside one needs-you session and one finished session; only actionable rows contribute to the correct count."
      - "Open the task from the OS bell, return to Home, then approve it; both rows disappear after authoritative refetch and exactly one run starts."
      - "Repeat with Reject and confirm no session row, title count, or task approval survives stale in another tab."
      - "Disconnect the task query while session truth remains live; the bell must name the degraded source rather than inventing a combined count."
    must_avoid:
      - "Do not broaden this canary into Loop approval gates or settle it from session-only fixtures."
```

<!-- Durable adjacent canary for the Herdr parity targeted cycle; every run debrief belongs in its dated report. -->
