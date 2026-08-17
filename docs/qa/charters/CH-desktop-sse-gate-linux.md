# CH-desktop-sse-gate-linux: Ten-minute SSE streaming-load release gate in Electron

```yaml
charter:
  id: CH-desktop-sse-gate-linux
  mission: "As Théo on Linux, hold the most stream-heavy product screens open in Electron for 10 minutes, measuring concurrent SSE/WS connections and UI liveness — this recorded QA gate blocks shipment on any failure."
  mode: strategy-based
  platform: linux
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-desktop-attach-daily
  scenarios: []
  tour: Network Tour
  time_box_minutes: 60
  e2e: []
  lab:
    bootstrap: "eng-qa-bootstrap — fresh bootstrap-manifest.json for this pass; seed enough live sessions/runs to saturate the stream-heavy screens"
    isolation: "unique COMPOZY_HOME + daemon ports from the manifest; default home/ports forbidden"
    web_proxy: "any companion browser measurement derives COMPOZY_WEB_API_PROXY_TARGET from the manifest; never hardcode :2123"
    pids: "register lab daemon and stream generators at <QA_OUTPUT_PATH>/qa/pids/<name>.pid"
    config_writes: "config mutations run sequentially per QA home — never parallel"
    teardown: "eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every terminal path; cite teardown.json \"clean\": true (L-029)"
  guidance:
    must_try:
      - "Open the most stream-heavy screens with multiple live producers; hold for a continuous 10-minute window in the packaged Electron Chromium renderer."
      - "Measure the per-origin concurrent SSE/WS connection profile and record it into the release evidence; note any Electron connection ceiling or starvation behavior verbatim for the support runbook."
      - "Assert UI liveness: no starved stream, no dead pane, no permanently stalled screen; interact mid-window and confirm streams survive."
      - "Record verdict as a release-gate result (pass/fail + profile) in the run report and release record; this charter settles no tracker scenario."
    must_avoid:
      - "No synthetic pass from short windows — the 10-minute continuous hold is the contract."
      - "Never run against the operator's default home; never leave stream generators alive past teardown."
```

<!-- Immutable charter: each run's debrief belongs in that run's dated report. -->
