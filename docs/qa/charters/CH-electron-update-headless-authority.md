# CH-electron-update-headless-authority: Drive the unified update authority without a UI

```yaml
charter:
  id: CH-electron-update-headless-authority
  mission: "As Ada, drive status, open, retry, diagnose, update, cancel, settings transports, and update cadence from structured surfaces alone, proving one fenced operation and no surviving app-scoped update alias."
  mode: strategy-based
  platform: macos and linux
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-desktop-agent-headless
  scenarios: [APP-agent-cli-app-verbs, APP-single-command-multi-target-update, APP-cancel-dormant-update, APP-update-config-cadence]
  tour: Feature Tour
  time_box_minutes: 90
  cadence_tier: targeted
  hot_spots:
    safety_invariants: [1, 2, 3, 4, 5, 7, 8, 9, 10, 17, 18]
    adrs: [ADR-002, ADR-009]
  guidance:
    must_try:
      - "Validate every `compozy app status|open|retry|diagnose -o json` state and error against schema v2, including absent and unresponsive control sockets plus consent-gated diagnostic export."
      - "Run `compozy update --check`, apply, and `--cancel` beside settings GET/apply/cancel over HTTP and UDS; compare operation id, holder, revision, target order, phases, errors, and persisted journal after every transition."
      - "Invoke the deleted app-scoped updater subcommand and prove unknown-command output, no alias, no record, no channel request, and no filesystem mutation."
      - "Change `[app].update_check` and interval sequentially through file, CLI, HTTP/UDS config, and native config tools; prove live daemon cadence, validation, persistence, and zero shell ownership."
    must_avoid:
      - "Do not infer any result from the UI or from source code; terminal transcripts and independent structured readbacks own this session."
      - "Never issue concurrent config writes against one home or reuse the operator's control socket/token."
```

<!-- Immutable charter: each run's debrief belongs in its dated report. -->
