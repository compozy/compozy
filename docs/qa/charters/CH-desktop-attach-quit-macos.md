# CH-desktop-attach-quit-macos: Attach, start, crash, and quit contracts on macOS (scripted-manual smoke)

```yaml
charter:
  id: CH-desktop-attach-quit-macos
  mission: "As Dora on macOS, prove the app is a second door onto my runtime — attach untouched, start when stopped, degrade honestly on crash, and quit without ever stopping the runtime — scripted-manual with recorded evidence (no WebDriver on macOS)."
  mode: scenario-based
  platform: macos
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
      - "E2E-003/E2E-018: daemon running with an active session + browser tab → open app → identical state, same origin, no second daemon; act in each surface and watch the other reflect live."
      - "E2E-004: stopped runtime → open → starting progress → product; quit → `compozy status` healthy, session survives; force-kill the app and prove the next launch attaches normally."
      - "E2E-006: kill -9 the daemon under the app → disconnected state within the interval → restart affordance → product returns with preserved state; E2E-005: pin an old runtime → guided skew state naming both versions."
      - "E2E-010: move/resize → quit → relaunch restores geometry; then a disconnected-display profile → centered recovery. Platform smoke: macOS logout/login with the app open (US-008.EC-1) → post-login `compozy status` shows no app-initiated stop."
    must_avoid:
      - "Never signal, restart, or stop the lab daemon through anything but the scripted fault injections; the CLI remains the only legitimate stop surface."
      - "No WebDriver claims on macOS; never leave the lab daemon or browser alive past teardown."
```

<!-- Immutable charter: each run's debrief belongs in that run's dated report. -->
