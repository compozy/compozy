# CH-extension-command-authority: Prove contributed commands are only a safe presentation layer

```yaml
charter:
  id: CH-extension-command-authority
  mission: "As Bruno, discover and execute extension-contributed commands through human and structured surfaces, proving malformed metadata fails before runtime and every valid leaf performs exactly one canonical tool invocation under unchanged policy, approval, availability, and workspace scope."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-run-extension-commands
  scenarios: [ET-discover-extension-command-tree, ET-run-extension-projected-flags, ET-run-extension-command-raw-input, ET-enforce-extension-command-approval, ET-refuse-extension-command-group, ET-extension-manifest-v2-surfaces, ET-profile-workspace-tool-isolation]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Build command metadata with depth, collision, reserved-flag, empty-group, and unsupported-schema failures; prove both build and manifest load reject the malformed tree before discovery."
      - "Compare `compozy extension commands` human/JSON with HTTP and UDS `GET /api/extensions/commands`, including group presentation, flat leaves, projected flags, risk, approval, and workspace filtering."
      - "Invoke one leaf using every supported scalar/nullable/enum/array projected flag and again with raw --input; mixing forms, invalid JSON, a group, or an unknown path must fail before any runtime call."
      - "Exercise approval-required and unavailable leaves against equivalent `tool invoke`. Count calls: each accepted extension exec must issue exactly one canonical tool invocation with identical trusted workspace and result."
    must_avoid:
      - "Direct subprocess invocation, treating a group as executable, bypassing approval for convenience, or accepting visually similar human output without structured field parity."
  evidence_expectations:
    - "Build/load rejection matrix, discovery payload diffs across CLI/HTTP/UDS, and canonical invocation counters for projected/raw/approved/unavailable paths."
    - "Workspace and approval-token captures proving extension exec and tool invoke share one authority path."
```

## Selection rationale

Targeted tier. ADR-008 is the sole command-surface architecture decision. Safety Invariants 16–17
are end-to-end assertions here: metadata cannot create authority, and malformed paths/flags/schema
projections cannot reach runtime.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
