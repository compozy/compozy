# CH-desktop-links-instance-windows: Deep links, single instance, and link boundaries on Windows

```yaml
charter:
  id: CH-desktop-links-instance-windows
  mission: "As Théo on Windows, prove compozyos:// links land in one window on the right view — running, cold, forwarded from a second launch — and that hostile payloads and external links never escape the boundary."
  mode: charter-with-tour
  platform: windows
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-desktop-link-driven
  scenarios: [APP-deep-link-running, APP-deep-link-cold-start, APP-single-instance-focus]
  tour: Garbage Tour
  time_box_minutes: 60
  e2e: [E2E-007, E2E-008, E2E-009, E2E-023]
  lab:
    bootstrap: "eng-qa-bootstrap — fresh bootstrap-manifest.json for this pass; reuse only while the same QA loop is active"
    isolation: "unique COMPOZY_HOME + daemon ports from the manifest; default home/ports forbidden"
    web_proxy: "derive COMPOZY_WEB_API_PROXY_TARGET from the manifest if any browser surface is used; never hardcode :2123"
    pids: "register lab daemon at <QA_OUTPUT_PATH>/qa/pids/<name>.pid"
    config_writes: "config mutations run sequentially per QA home — never parallel"
    teardown: "eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every terminal path; cite teardown.json \"clean\": true (L-029)"
  guidance:
    must_try:
      - "E2E-008: registered scheme activation (`start compozyos://open/<existing-session-path>`) with the app running → focus + correct view; deleted entity → not-found view; malformed/hostile payloads → default view, no dialog."
      - "E2E-023: app closed → link activation cold-starts, runs needed states, renders the linked view; single navigation in the app log."
      - "E2E-007: second launch from Start menu/taskbar/installer 'open'/`compozy app open` → existing window focused, process count unchanged; second launch with a link argument → forwarded; also crash the app and prove the stale single-instance state recovers (US-009.EC-2)."
      - "E2E-009: external https + `target=_blank` → OS default browser, app stays on the product."
    must_avoid:
      - "No forced actionability bypasses in the Playwright _electron test."
      - "Never run against the operator's default home; never leave lab processes alive past teardown."
```

<!-- Immutable charter: each run's debrief belongs in that run's dated report. -->
