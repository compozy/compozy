# CH-herdr-interaction-recovery: Resolve live and restart-orphaned requests exactly once

```yaml
charter:
  id: CH-herdr-interaction-recovery
  mission: "As Théo, answer another session's permission and clarification before and after restart and prove one durable winner resumes work without cross-workspace or self-action leakage."
  mode: charter-with-tour
  cycle_tier: targeted
  cycle_role: primary
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-answer-agent-requests
  scenarios: [RT-021, RT-session-clarification-roundtrip, RT-session-native-interaction-resolution]
  tour: Interrupt Tour
  time_box_minutes: 60
  adrs: [ADR-001, ADR-005]
  safety_invariants: [2, 3, 9, 11, 14, 15]
  guidance:
    must_try:
      - "Resolve one live permission and one live clarification through different Web, CLI, HTTP/UDS, and native surfaces; compare interaction id, actor, decision, and transcript receipt."
      - "Restart with both request kinds pending, resolve each native interaction once, and race a duplicate caller; the original winner must be returned."
      - "Fill the continuation queue before orphan resolution; queue-full must leave the pending record untouched and retryable."
      - "Attempt self and neighboring-workspace resolution and inspect every exposed title, payload, result, log, hook, and prompt for bounded redacted content."
    must_avoid:
      - "Do not approve the owning session's own request or bypass public interaction discovery with database reads."
```

<!-- Durable targeted charter; every run debrief belongs in its dated report. -->
