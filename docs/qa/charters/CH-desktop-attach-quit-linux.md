# CH-desktop-attach-quit-linux: Attach, start, crash, and quit contracts on Linux

```yaml
charter:
  id: CH-desktop-attach-quit-linux
  mission: "As Dora on Linux, prove through Playwright _electron that attach stays untouched, a stopped runtime starts, crashes degrade honestly, and quit never stops the runtime."
  mode: scenario-based
  platform: linux
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-desktop-attach-daily
  scenarios: [APP-attach-running-daemon, APP-start-installed-daemon, APP-quit-contract]
  tour: Interrupt Tour
  time_box_minutes: 90
  e2e: [E2E-003, E2E-004, E2E-005, E2E-006, E2E-010, E2E-018]
  lab:
    bootstrap: "eng-qa-bootstrap — fresh bootstrap-manifest.json for this pass; reuse only while the same QA loop is active"
    isolation: "unique COMPOZY_HOME + daemon ports from the manifest; default home/ports forbidden"
    web_proxy: "browser-coexistence steps (E2E-003/E2E-018) derive COMPOZY_WEB_API_PROXY_TARGET from the manifest; never hardcode :2123"
    pids: "register lab daemon and any browser at <QA_OUTPUT_PATH>/qa/pids/<name>.pid"
    config_writes: "config mutations run sequentially per QA home — never parallel"
    teardown: "eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every terminal path; cite teardown.json \"clean\": true (L-029)"
  guidance:
    must_try:
      - "E2E-003/E2E-018: running daemon + browser tab → open app → identical state, no second daemon; two-way live sync through the manifest-derived proxy target."
      - "E2E-004: stopped runtime → open → starting progress → product; quit → `compozy status` healthy; SIGKILL the app → next launch attaches normally; also race a terminal `compozy daemon start` against the app start (US-004.EC-3) → exactly one runtime."
      - "E2E-006: kill -9 the daemon → disconnected state → restart affordance → recovery; E2E-005: pinned old runtime → guided skew state naming both versions."
      - "E2E-010: geometry persistence + disconnected-display (virtual display profile) recovery. Platform smoke: session logout with the app open (US-008.EC-1)."
    must_avoid:
      - "No forced actionability bypasses; the CLI remains the only legitimate stop surface."
      - "Never run against the operator's default home or ports; never leave lab processes alive past teardown."
```

<!-- Immutable charter: each run's debrief belongs in that run's dated report. -->
