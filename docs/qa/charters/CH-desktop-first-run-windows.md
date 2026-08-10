# CH-desktop-first-run-windows: Newcomer install and first run on Windows

```yaml
charter:
  id: CH-desktop-first-run-windows
  mission: "As Lea on a clean Windows lab home, prove signed install → guided provisioning → working product with honest failure branches, scripted via tauri-driver plus manual installer semantics."
  mode: scenario-based
  platform: windows
  persona:
    name: Lea
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-desktop-first-run
  scenarios: [APP-install-first-run-provision, APP-brand-channel-visibility]
  tour: Feature Tour
  time_box_minutes: 60
  e2e: [E2E-001, E2E-002, E2E-011, E2E-016, E2E-020]
  lab:
    bootstrap: "eng-qa-bootstrap — fresh bootstrap-manifest.json for this pass; reuse only while the same QA loop is active"
    isolation: "unique COMPOZY_HOME + daemon ports from the manifest; default home/ports forbidden"
    web_proxy: "derive COMPOZY_WEB_API_PROXY_TARGET from the manifest if any browser surface is used; never hardcode :2123"
    pids: "register long-lived lab processes at <QA_OUTPUT_PATH>/qa/pids/<name>.pid on spawn"
    config_writes: "config mutations run sequentially per QA home — never parallel"
    teardown: "eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every terminal path; cite teardown.json \"clean\": true (L-029)"
  guidance:
    must_try:
      - "E2E-001: signed installer (no SmartScreen unsigned wall — capture it) → open → staged provisioning → product UI → quit → relaunch direct; one daemon in the process table, `compozy status` healthy."
      - "E2E-002: offline first run → honest retry state → network back → same-screen retry completes."
      - "E2E-011: WebView2 launch frame-capture (no white flash); wedge the daemon → bounded loading → honest error state."
      - "E2E-020: reinstall N over N-1 → exactly one uninstall registry record; E2E-016: About shows CompozyOS + beta + version, no stable selector."
    must_avoid:
      - "No forced actionability bypasses in the tauri-driver script."
      - "Never run against the operator's default home or ports; never leave the lab daemon alive past teardown."
```

<!-- Immutable charter: each run's debrief belongs in that run's dated report. -->
