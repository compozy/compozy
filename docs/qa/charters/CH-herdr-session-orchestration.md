# CH-herdr-session-orchestration: Control sibling sessions without polling or shell access

```yaml
charter:
  id: CH-herdr-session-orchestration
  mission: "As Ada, inspect, wait on, notify, cancel, and stop sibling sessions through structured surfaces and prove every race ends in one bounded deterministic outcome."
  mode: strategy-based
  cycle_tier: targeted
  cycle_role: primary
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-15
  scenarios: [RT-session-attention-catalog, RT-042, RT-operator-notification-delivery, RT-session-wait-state, RT-session-prompt-cancel, RT-session-native-stop]
  tour: Interrupt Tour
  time_box_minutes: 60
  adrs: [ADR-001, ADR-002, ADR-005]
  safety_invariants: [4, 5, 8, 9, 11, 14]
  guidance:
    must_try:
      - "Compare CLI JSON, HTTP, UDS, and native outputs for attention lists, summary totals, interactions, waits, notify outcomes, prompt cancellation, and stop."
      - "Interrupt waits at subscribe/snapshot, timeout/resume, delete/stop, buffer overflow, caller cancel, and cap boundaries; no edge or registration may disappear silently."
      - "Cancel one in-flight prompt and prove the session accepts a later prompt; stop another through compozy__session_stop and race a second surface."
      - "Invoke all seven native tools across the cycle, including notify and the six session tools; self, foreign-workspace, approval-gated, and unavailable paths must return their documented reason."
    must_avoid:
      - "Do not poll the database, widen an agent to operator scope, or treat a 2xx transport response as proof of the wrong structured outcome."
```

<!-- Durable targeted charter; every run debrief belongs in its dated report. -->
