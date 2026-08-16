# CH-herdr-attention-accessibility: Act on attention without sight or a pointer

```yaml
charter:
  id: CH-herdr-attention-accessibility
  mission: "As Sol, read and act on session attention by keyboard and screen reader and prove no state, count, filter, or recovery path depends on color or pointer precision."
  mode: charter-with-tour
  cycle_tier: targeted
  cycle_role: primary
  persona:
    name: Sol
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-respond-to-agent-attention
  scenarios: [MS-web-attention-channel-states, RT-web-attention-bell-jump, RT-web-session-all-workspaces, RT-web-session-attention-sort]
  tour: Back-Button Tour
  time_box_minutes: 60
  adrs: [ADR-001, ADR-002]
  safety_invariants: [1, 12, 16]
  visual_contract: "docs/design/opendesign/herdr-parity/: task_03 VC-01..VC-16 and VC-21..VC-23"
  guidance:
    must_try:
      - "Traverse every badge, bell section, task row, workspace group, sort, and channel state by keyboard; the announced name must distinguish input, auth, failure, done, muted, stale, denied, and unavailable."
      - "Open and close the bell and Settings routes with Escape and browser Back; focus must return to a sensible trigger without clearing done early."
      - "Switch to All workspaces, collapse and retry groups, then land on a foreign session using only the keyboard and confirm the destination receives focus."
      - "Enable reduced motion and verify attention transitions remain perceivable without animation or color alone."
    must_avoid:
      - "Do not replace the real screen-reader transcript with DOM inspection or treat board color parity as accessibility evidence."
```

<!-- Durable targeted accessibility charter; every run debrief belongs in its dated report. -->
