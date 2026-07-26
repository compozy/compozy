# CH-network-admin-lifecycle: Govern Live policy and extension requirements without enrollment

```yaml
charter:
  id: CH-network-admin-lifecycle
  mission: "As Bruno, change Network availability and finite Live policy, disable and re-enable around real work, and confirm Network-aware bundle/extension requirements without any setting, channel declaration, or hook silently enrolling an execution."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-administer-network-live
  scenarios: [NB-network-live-config-lifecycle, NB-network-availability-toggle, NB-001, NB-002, MS-037, ET-025, ET-026, ET-027, ET-028, ET-030, ET-network-participation-hooks, NB-023]
  tour: Multi-Tab Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Bootstrap a fresh isolated lab with unique AGH_HOME/ports/provider home/tmux socket, register PIDs, export AGH_WEB_API_PROXY_TARGET, and execute eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every terminal path; cite teardown.json clean=true."
      - "Use Playwright/browser-use for Network Settings, save/dirty/restart states, ready/disabled/active summaries, and bundle activation confirmation at 375/768/1280 widths with keyboard-visible focus. Open the same settings/activation in two tabs and verify last-write/version conflict behavior without hiding the final state."
      - "Apply config writes sequentially in the one QA home: valid Live durations/budgets, reload/restart round trip, removed keys, and over-ceiling values. Compare Web, config.toml, agh config/network -o json, HTTP, and UDS before and after restart."
      - "Disable around one active Live wake and one Local control, re-enable, then preview/activate/update a Live-requiring bundle. Exercise missing confirmation, repeated confirmation, changed digest, declared-channel inventory, and a real pre_resolve hook that allows, denies, narrows, and attempts to widen."
    must_avoid:
      - "Parallel config writes against the same AGH_HOME, auto-confirming a changed digest, interpreting channel inventory as participation, or testing mailbox/remote transport/configurable spend caps."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief belongs in the dated report. -->
