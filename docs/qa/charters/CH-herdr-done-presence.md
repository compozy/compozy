# CH-herdr-done-presence: Race independent viewers against finished-unseen truth

```yaml
charter:
  id: CH-herdr-done-presence
  mission: "As Théo with two browser clients, let a session settle in and out of view and prove only a live owned presence lease prevents or clears done."
  mode: charter-with-tour
  cycle_tier: targeted
  cycle_role: primary
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-11
  scenarios: [RT-session-done-presence]
  tour: Multi-Tab Tour
  time_box_minutes: 60
  adrs: [ADR-001, ADR-005]
  safety_invariants: [1, 3, 12, 14, 16]
  guidance:
    must_try:
      - "Acquire two independent leases, renew and release each only with its own lease id, then settle while either, both, or neither remains live."
      - "Let one lease expire while the tab is hidden and prove a later settle derives done; passive CLI, HTTP, and UDS reads must not clear it."
      - "Focus the session to clear done, then compare catalog wake before attention event and fresh reads in both clients."
      - "Restart the daemon between settle and view; the revision fence and finished-unseen state must survive."
    must_avoid:
      - "Do not call presence with an agent identity or treat a local UI cache change as a daemon verdict."
```

<!-- Durable targeted charter; every run debrief belongs in its dated report. -->
