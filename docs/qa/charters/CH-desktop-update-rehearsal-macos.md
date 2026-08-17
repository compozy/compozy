# CH-desktop-update-rehearsal-macos: App and runtime update rehearsal with forced failure on macOS (scripted-manual smoke)

```yaml
charter:
  id: CH-desktop-update-rehearsal-macos
  mission: "As Bruno on macOS, rehearse the whole update moment against a mock GitHub Release and channel-beta fixture — background app update, consented restart, runtime update under in-flight work, managed recommendation, and a forced apply failure that must leave the app intact — scripted-manual with recorded evidence."
  mode: scenario-based
  platform: macos
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
      - "E2E-016/E2E-020: install N → publish N+1 through the local release authority into the mock GitHub Release and channel-beta fixture → consented installer handoff or closed-app staging → N+1 verified after launch."
      - "E2E-017 (forced failure): lock the install dir → apply fails → failed-update report + manual-download path opens the release page; the OS-level app remains launchable and permissions stay unchanged — capture `ls -l` before/after."
      - "E2E-018: app-owned runtime + agent work in flight → timing consent; 'later' keeps working; 'now' quiesces, applies, reconnects. A post-swap health failure restores the previous runtime and records `rolled-back`."
      - "E2E-019/E2E-021/E2E-026: compare Settings, indicator, CLI, HTTP, and UDS; a Homebrew runtime returns the exact `brew` recommendation with zero binary writes. Sleep/wake across a check cycle must not duplicate prompts."
      - "Abandoned-install walk: launch an obsolete app build after its referenced channel generation is unavailable; record the failed check and verify the manual GitHub Release recovery path without restoring any retired feed."
    must_avoid:
      - "Never point the updater at the production GitHub release/channel authority or sign fixtures with production credentials."
      - "Never leave the mock GitHub server, lab daemon, or a half-applied update alive past teardown."
```

<!-- Immutable charter: each run's debrief belongs in that run's dated report. -->
