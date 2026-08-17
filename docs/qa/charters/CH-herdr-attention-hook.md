# CH-herdr-attention-hook: Observe committed attention edges without changing them

```yaml
charter:
  id: CH-herdr-attention-hook
  mission: "As Ada, discover the new attention hook, observe one committed edge through a real extension, and prove observer mutation or failure cannot alter runtime truth."
  mode: charter-with-tour
  cycle_tier: targeted
  cycle_role: primary
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-agent-marketplace-parity
  scenarios: [ET-042]
  tour: Feature Tour
  time_box_minutes: 30
  adrs: [ADR-005]
  safety_invariants: [3, 11, 14]
  guidance:
    must_try:
      - "Discover session.attention.changed through HTTP, UDS, CLI, and compozy__hooks_events and compare its async-only typed descriptor."
      - "Install or enable a real extension observer, drive one badge edge, and capture from, to, class, at, session, and workspace after the canonical commit."
      - "Mutate the observer's payload copy and throw from another observer; runtime state, later observers, catalog wake, and attention delivery must remain correct."
      - "Inspect the exposed payload and logs for bounded redacted content and workspace ownership."
    must_avoid:
      - "Do not substitute a unit-test spy for the extension observation or use the hook to mutate session state."
```

<!-- Durable targeted charter; every run debrief belongs in its dated report. -->
