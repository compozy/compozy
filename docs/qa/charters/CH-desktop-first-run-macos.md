# CH-desktop-first-run-macos: Newcomer install and first run on macOS (scripted-manual smoke)

```yaml
charter:
  id: CH-desktop-first-run-macos
  mission: "As Lea on a clean macOS lab home, prove download → open → guided provisioning → working product with honest failure branches — scripted-manual because macOS has no WebDriver, every step recorded."
  mode: scenario-based
  platform: macos
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
      - "E2E-001: signed dmg install (Gatekeeper acceptance on record) → open → every provisioning stage visible → product UI → quit → relaunch lands directly in product; capture process table (one daemon) and `compozy status`."
      - "E2E-002: start offline → honest network-required state → enable network → retry from the same screen completes provisioning."
      - "E2E-011: frame-capture the launch (no white flash, branded loading); SIGSTOP the daemon → bounded loading hands off to the honest error state with diagnostics access."
      - "E2E-020: install build N over N-1 → single app entry, no duplicate identity; E2E-016: About shows CompozyOS + beta + version, no stable selector."
    must_avoid:
      - "No WebDriver/automation claims on macOS — every verdict rides the recorded manual walk."
      - "Never run against the operator's default home, ports, or provider login; never leave the lab daemon alive past teardown."
```

<!-- Immutable charter: each run's debrief belongs in that run's dated report. -->
