# CH-desktop-agent-headless-cli: Drive the whole desktop lifecycle through compozy app with no human

```yaml
charter:
  id: CH-desktop-agent-headless-cli
  mission: "As Ada, operate the desktop surface end to end through `compozy app` structured verbs alone — install truth, launch, transitional states, updates, recovery, diagnostics — every payload schema-valid and every error deterministically named."
  mode: scenario-based
  platform: linux (primary scripted run; socket-error and status branches re-walked on macOS)
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-desktop-agent-headless
  scenarios: [APP-agent-cli-app-verbs]
  tour: Feature Tour
  time_box_minutes: 60
  e2e: [E2E-024, E2E-025, E2E-026, E2E-027]
  lab:
    bootstrap: "eng-qa-bootstrap — fresh bootstrap-manifest.json for this pass; mock GitHub Release and channel-beta fixture for the update legs"
    isolation: "unique COMPOZY_HOME + daemon ports from the manifest; default home/ports forbidden"
    web_proxy: "not applicable unless a browser cross-check is added; if so, derive COMPOZY_WEB_API_PROXY_TARGET from the manifest"
    pids: "register the lab daemon, app process, and mock GitHub server at <QA_OUTPUT_PATH>/qa/pids/<name>.pid"
    config_writes: "config mutations run sequentially per QA home — never parallel"
    teardown: "eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every terminal path; cite teardown.json \"clean\": true (L-029)"
  guidance:
    must_try:
      - "E2E-024/E2E-025: status before install (installed:false, exit 0) → install → `compozy app open` → status through provisioning → attached → `open /settings` focuses and navigates → kill the app → running:false; validate every `-o json` payload against desktop/schema/app-state.schema.json."
      - "E2E-026/E2E-027: `compozy update --check -o json` reports the mock channel availability; the multi-target path runs runtime first and stages or hands off the app, while the headless path omits the app target."
      - "Force a post-swap runtime health failure and prove the operation archives `rolled-back`, the previous binary is restored byte-identically, and a retry requires a newly verified candidate."
      - "Probe the failure vocabulary: `app_not_installed`, `invalid_target_path`, socket absent → `app_not_running`, socket unresponsive → `app_control_unavailable`; assert socket permissions 0600."
    must_avoid:
      - "Never infer state from the UI — this session is terminal-only; screenshots are not evidence here, transcripts are."
      - "Never run against the operator's default home; never leave the app, daemon, or mock GitHub server alive past teardown."
```

<!-- Immutable charter: each run's debrief belongs in that run's dated report. -->
