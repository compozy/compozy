# CH-desktop-update-rehearsal-linux: App and runtime update rehearsal with forced failure on Linux

```yaml
charter:
  id: CH-desktop-update-rehearsal-linux
  mission: "As Bruno on Linux, rehearse the update moment against the staging fixture feed — background app update, consented restart, runtime update under in-flight work, managed recommendation, and a forced apply failure that must leave the app intact — scripted via tauri-driver."
  mode: scenario-based
  platform: linux
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-desktop-update-moment
  scenarios: [APP-app-auto-update, APP-runtime-update-app-owned, APP-runtime-update-managed, APP-update-recovery-state, APP-brand-channel-visibility]
  tour: Interrupt Tour
  time_box_minutes: 90
  e2e: [E2E-012, E2E-013, E2E-014, E2E-015, E2E-016, E2E-022, E2E-025]
  lab:
    bootstrap: "eng-qa-bootstrap — fresh bootstrap-manifest.json for this pass; staging minisign keypair + local fixture feed only, never the production key"
    isolation: "unique COMPOZY_HOME + daemon ports from the manifest; default home/ports forbidden"
    web_proxy: "derive COMPOZY_WEB_API_PROXY_TARGET from the manifest if any browser surface is used; never hardcode :2123"
    pids: "register lab daemon and the fixture feed server at <QA_OUTPUT_PATH>/qa/pids/<name>.pid"
    config_writes: "config mutations (update_check, feed overrides) run sequentially per QA home — never parallel"
    teardown: "eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every terminal path; cite teardown.json \"clean\": true (L-029)"
  guidance:
    must_try:
      - "E2E-012: install N → publish N+1 to the fixture feed → background download → 'ready' → consent restart → N+1 running; E2E-022: quit with the update pending → applied on next launch per platform convention."
      - "E2E-013 (forced failure): make the install path read-only → apply fails → failed-update report + manual-download path; the app must remain launchable and the install path mode bits unchanged — never left `0700` by the failed apply; capture `stat` before/after."
      - "E2E-014: app-owned runtime + in-flight work → timing consent → 'now' walks quiesce (drain → safe-to-stop → revalidate) → swap → reconnect; verify a corrupt archive fails BEFORE any quiesce or stop (IT-017 posture) — the daemon is never drained for an unverifiable artifact."
      - "E2E-015: managed/PATH install → exact channel recommendation, zero binary writes; E2E-025: recovery_required journey; E2E-016: About shows beta + versions, no stable selector; also flip `update_check=false` → zero feed hits observed (IT-025 posture)."
    must_avoid:
      - "Never point the updater at the production feed or sign fixtures with the production key."
      - "Never leave the fixture feed server, lab daemon, or a half-applied update alive past teardown."
```

<!-- Immutable charter: each run's debrief belongs in that run's dated report. -->
