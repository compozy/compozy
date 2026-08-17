# CH-desktop-update-rehearsal-linux: App and runtime update rehearsal with forced failure on Linux

```yaml
charter:
  id: CH-desktop-update-rehearsal-linux
  mission: "As Bruno on Linux, rehearse the update moment against a mock GitHub Release and channel-beta fixture — background app update, consented restart, runtime update under in-flight work, managed recommendation, and a forced apply failure that must leave the app intact — through Playwright _electron."
  mode: scenario-based
  platform: linux
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-desktop-update-moment
  scenarios: [APP-app-auto-update, APP-runtime-update-app-owned, APP-runtime-update-managed, APP-update-recovery-state, APP-brand-channel-visibility, APP-abandoned-install-update-polling]
  tour: Interrupt Tour
  time_box_minutes: 90
  e2e: [E2E-016, E2E-017, E2E-018, E2E-019, E2E-020, E2E-021, E2E-026]
  lab:
    bootstrap: "eng-qa-bootstrap — fresh bootstrap-manifest.json for this pass; mock GitHub Release and channel-beta fixtures only, never a production token"
    isolation: "unique COMPOZY_HOME + daemon ports from the manifest; default home/ports forbidden"
    web_proxy: "derive COMPOZY_WEB_API_PROXY_TARGET from the manifest if any browser surface is used; never hardcode :2123"
    pids: "register the lab daemon and mock GitHub server at <QA_OUTPUT_PATH>/qa/pids/<name>.pid"
    config_writes: "config mutations such as update_check run sequentially per QA home — never parallel"
    teardown: "eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every terminal path; cite teardown.json \"clean\": true (L-029)"
  guidance:
    must_try:
      - "E2E-016/E2E-020: install N → publish N+1 through the local release authority into the mock GitHub Release and channel-beta fixture → background download → consented installer handoff or closed-app staging → N+1 verified after launch."
      - "E2E-017 (forced failure): make the install path read-only → apply fails → failed-update report + manual-download path; the app must remain launchable and the install path mode bits unchanged — never left `0700` by the failed apply; capture `stat` before/after."
      - "E2E-018: app-owned runtime + in-flight work → timing consent → quiesce (drain → safe-to-stop → revalidate) → swap → reconnect; a corrupt archive fails before quiesce, and a post-swap health failure restores the previous runtime and records `rolled-back`."
      - "E2E-019/E2E-021/E2E-026: compare Settings, indicator, CLI, HTTP, and UDS; managed/PATH installs return the exact channel recommendation with zero binary writes. Set `update_check=false` and observe zero GitHub channel reads."
      - "Abandoned-install walk: launch an obsolete app build after its referenced channel generation is unavailable; record the failed check and verify the manual GitHub Release recovery path without restoring any retired feed."
    must_avoid:
      - "Never point the updater at the production GitHub release/channel authority or sign fixtures with production credentials."
      - "Never leave the mock GitHub server, lab daemon, or a half-applied update alive past teardown."
```

<!-- Immutable charter: each run's debrief belongs in that run's dated report. -->
