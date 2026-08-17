# CH-herdr-attention-signals: Follow one attention edge across every operator signal

```yaml
charter:
  id: CH-herdr-attention-signals
  mission: "As Cora, leave several sessions working across workspaces, follow each new attention edge through the configured signals, and land on the exact work without stale counts or a lost task approval."
  mode: charter-with-tour
  cycle_tier: targeted
  cycle_role: primary
  persona:
    name: Cora
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-respond-to-agent-attention
  scenarios: [MS-web-attention-channel-states, RT-session-spawn-wake, RT-web-attention-bell-jump, RT-web-attention-title-count, RT-web-attention-toast-delivery, RT-web-session-all-workspaces, RT-web-session-attention-sort]
  tour: Feature Tour
  time_box_minutes: 60
  adrs: [ADR-001, ADR-002, ADR-005]
  safety_invariants: [1, 3, 6, 7, 8, 11, 13, 14, 16]
  visual_contract: "docs/design/opendesign/herdr-parity/: task_03 VC-01..VC-23"
  guidance:
    must_try:
      - "Create input, permission, failure, and finished edges in two workspaces; compare sidebar, bell, title, toast, sound, system channel, and the Sessions scope/sort."
      - "Keep the app focused for one edge, hide it for another, mute one workspace, and resolve one row before clicking its toast; no suppressed or stale event may replay."
      - "Activate same-workspace and foreign-workspace rows, including a closed session window; the shared jump must wait for the workspace barrier and focus the exact session."
      - "Keep one pending task approval beside the session rows and prove its row and task-backed count survive the attention rewrite."
    must_avoid:
      - "Do not infer notification delivery from event publication alone or use Storybook as the persona verdict."
      - "Do not judge prototype copy or host chrome; use the named boards only for the in-scope visual language."
```

<!-- Durable targeted charter for the Herdr parity attention surfaces. Run debriefs belong only in the dated report. -->
