# CH-desktop-sse-gate-windows: Ten-minute SSE streaming-load release gate on WebView2

```yaml
charter:
  id: CH-desktop-sse-gate-windows
  mission: "As Théo on Windows, hold the most stream-heavy product screens open in the app for 10 minutes on WebView2, measuring concurrent SSE/WS connections and UI liveness — E2E-021 is a release gate: a failure blocks ship and escalates per TechSpec Known Risks."
  mode: strategy-based
  platform: windows
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-desktop-attach-daily
  scenarios: []
  tour: Network Tour
  time_box_minutes: 60
  e2e: [E2E-021]
  lab:
    bootstrap: "eng-qa-bootstrap — fresh bootstrap-manifest.json for this pass; seed enough live sessions/runs to saturate the stream-heavy screens"
    isolation: "unique COMPOZY_HOME + daemon ports from the manifest; default home/ports forbidden"
    web_proxy: "any companion browser measurement derives COMPOZY_WEB_API_PROXY_TARGET from the manifest; never hardcode :2123"
    pids: "register lab daemon and stream generators at <QA_OUTPUT_PATH>/qa/pids/<name>.pid"
    config_writes: "config mutations run sequentially per QA home — never parallel"
    teardown: "eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every terminal path; cite teardown.json \"clean\": true (L-029)"
  guidance:
    must_try:
      - "Open the most stream-heavy screens with multiple live producers; hold for a continuous 10-minute window on WebView2."
      - "Measure the per-origin concurrent SSE/WS connection profile and record it into the release evidence — WebView2 (Chromium) semantics may differ from the browser; the measured profile is the shipped truth."
      - "Assert UI liveness: no starved stream, no dead pane; interact mid-window and confirm streams survive screen switches and window minimize/restore."
      - "Record verdict as a release-gate result (pass/fail + profile) in the run report and release record; this charter settles no tracker scenario."
    must_avoid:
      - "No synthetic pass from short windows — the 10-minute continuous hold is the contract."
      - "Never run against the operator's default home; never leave stream generators alive past teardown."
```

<!-- Immutable charter: each run's debrief belongs in that run's dated report. -->
