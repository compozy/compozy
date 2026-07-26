# CH-network-local-default: Complete ordinary work with zero Network participation

```yaml
charter:
  id: CH-network-local-default
  mission: "As Nia, discover the Agent Network and complete ordinary session, task, Loop, and automation work while proving Local creates no hidden Network context, artifacts, activation, or usage."
  mode: charter-with-tour
  persona:
    name: Nia
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-network-local-default
  scenarios: [NB-execution-participation-defaults, NB-network-empties-onboarding-settings, NB-participation-controls-serialize, RT-010, RT-031, TA-001, TA-004, TA-007, TA-049]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Bootstrap one fresh isolated lab; use its unique AGH_HOME, daemon/Web ports, provider home, and tmux-bridge socket. Register long-lived PIDs, export AGH_WEB_API_PROXY_TARGET from the manifest, and run eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every pass/fail/blocked/abort exit; cite teardown.json with clean=true."
      - "Use Playwright/browser-use for onboarding, the ready and disabled Network empty states, Settings, and the session/task/Loop/automation controls at 375/768/1280 widths with keyboard-visible focus; shell/API evidence cannot settle those UI legs. Read the public Network guide and bundled AGH skill, then confirm those visits changed no setting."
      - "Record channel, wake, usage, status, and owner projections before and after one omitted-participation session, task run, Loop run, and schedule/webhook/trigger-backed automation fire. Re-read after refresh and daemon restart."
      - "Inspect Local agent context, prompt/environment/tool availability, then spawn child, review, and detached work. Each must resolve independently to Local; removed legacy fields must fail before mutation and preserve form input."
    must_avoid:
      - "Opting the main matrix into Live; testing mailbox, offline recipients, remote transport, or configurable spend caps; inspecting SQLite/source to decide a verdict."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief belongs in the dated report. -->
