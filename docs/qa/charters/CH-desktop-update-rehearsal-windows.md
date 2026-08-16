# CH-desktop-update-rehearsal-windows: App and runtime update rehearsal with forced failure on Windows

```yaml
charter:
  id: CH-desktop-update-rehearsal-windows
  mission: "As Bruno on Windows, rehearse the update moment against a mock GitHub release and channel — background app update, consented restart, runtime update under in-flight work, managed recommendation, and a forced apply failure that must leave the app intact — through Playwright _electron."
  mode: scenario-based
  platform: windows
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-desktop-update-moment
  scenarios: [APP-app-auto-update, APP-runtime-update-app-owned, APP-runtime-update-managed, APP-update-recovery-state, APP-brand-channel-visibility, APP-abandoned-install-update-polling]
  tour: Interrupt Tour
  time_box_minutes: 90
  e2e: [E2E-012, E2E-013, E2E-014, E2E-015, E2E-016, E2E-022, E2E-025]
  lab:
    bootstrap: "eng-qa-bootstrap — fresh bootstrap-manifest.json for this pass; mock GitHub release/channel fixtures only, never a production token"
    isolation: "unique COMPOZY_HOME + daemon ports from the manifest; default home/ports forbidden"
    web_proxy: "derive COMPOZY_WEB_API_PROXY_TARGET from the manifest if any browser surface is used; never hardcode :2123"
    pids: "register lab daemon and the fixture feed server at <QA_OUTPUT_PATH>/qa/pids/<name>.pid"
    config_writes: "config mutations (update_check, feed overrides) run sequentially per QA home — never parallel"
    teardown: "eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every terminal path; cite teardown.json \"clean\": true (L-029)"
  guidance:
    must_try:
      - "E2E-012: install N → publish N+1 to the fixture feed → background download → 'ready' → consent restart → N+1 running with the version indicator updated; E2E-022: quit with the update pending → the platform convention applies it on next launch, never losing it."
      - "E2E-013 (forced failure): lock the install dir (ACL fixture) → apply fails → failed-update report + manual-download path; the app must remain launchable from the OS and the install directory ACLs/permissions must be unchanged by the failed apply — capture `icacls` before/after."
      - "E2E-014: app-owned runtime + in-flight work → timing consent → 'now' quiesces/applies/reconnects, both versions in one surface; interrupt one download mid-cycle → partial artifact never applied (US-014.EC-2)."
      - "E2E-015: managed install → exact channel recommendation, zero binary writes; E2E-025: recovery_required journey with status/diagnose readouts; E2E-016: About shows beta + versions, no stable selector. Platform smoke: sleep/wake across a check cycle (US-014.EC-5)."
    must_avoid:
      - "Never point the updater at the production feed or sign fixtures with the production key."
      - "Never leave the fixture feed server, lab daemon, or a half-applied update alive past teardown."
```

<!-- Immutable charter: each run's debrief belongs in that run's dated report. -->
