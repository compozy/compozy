# CH-desktop-update-rehearsal-macos: App and runtime update rehearsal with forced failure on macOS (scripted-manual smoke)

```yaml
charter:
  id: CH-desktop-update-rehearsal-macos
  mission: "As Bruno on macOS, rehearse the whole update moment against the staging fixture feed — background app update, consented restart, runtime update under in-flight work, managed recommendation, and a forced apply failure that must leave the app intact — scripted-manual with recorded evidence (no WebDriver; E2E-012 runs here as the macOS release rehearsal)."
  mode: scenario-based
  platform: macos
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
      - "E2E-012: install N → publish N+1 to the fixture feed → background download → 'ready' → consent restart → N+1 running (macOS apply must honor the shell's own .app backup posture, UT-114); E2E-022: quit with the update pending → next launch applies it."
      - "E2E-013 (forced failure): lock the install dir → apply fails → failed-update report + manual-download path opens the release page; the OS-level app must remain launchable AND the install path permissions must be unchanged — never left `0700`/owner-only by the failed apply; capture `ls -l` before/after."
      - "E2E-014: app-owned runtime + agent work in flight → timing consent; 'later' keeps working; 'now' quiesces, applies, reconnects — both new versions in one surface. E2E-015: homebrew runtime → exact `brew` recommendation, zero binary writes (hash/mtime proof), surface clears after external update."
      - "E2E-025: induced post-migration boot failure → recovery_required sticky with typed error and diagnose log paths → fixed newer signed build clears it. E2E-016: About shows beta + versions, no stable selector. Platform smoke: sleep/wake across a check cycle (US-014.EC-5) → no duplicate prompts."
    must_avoid:
      - "Never point the updater at the production feed or sign fixtures with the production key."
      - "Never leave the fixture feed server, lab daemon, or a half-applied update alive past teardown."
```

<!-- Immutable charter: each run's debrief belongs in that run's dated report. -->
