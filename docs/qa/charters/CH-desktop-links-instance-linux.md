# CH-desktop-links-instance-linux: Deep links, single instance, and link boundaries on Linux

```yaml
charter:
  id: CH-desktop-links-instance-linux
  mission: "As Théo on Linux, prove through Playwright _electron that compozyos:// links land in one Chromium window on the right view — running, cold, forwarded — and that hostile payloads and external links never escape the boundary."
  mode: charter-with-tour
  platform: linux
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
      - "E2E-008: scheme activation (`xdg-open compozyos://open/<existing-session-path>`) with the app running → focus + correct view; deleted entity → not-found view; malformed/hostile payloads (scheme-in-path, traversal, encoded variants) → default view, no dialog."
      - "E2E-023: app closed → link activation cold-starts, runs needed states, renders the linked view; exactly one queued navigation fires."
      - "E2E-007: second launch from launcher/file manager/`compozy app open` → existing window focused, process count unchanged; rapid successive links → last one wins visibly (US-010.EC-3)."
      - "E2E-009: external https + `target=_blank` → OS default browser, app stays on the product; an internal-looking `http://localhost:<other-port>` is treated as external (US-011.EC-1)."
    must_avoid:
      - "No forced actionability bypasses in the Playwright _electron test."
      - "Never run against the operator's default home; never leave lab processes alive past teardown."
```

<!-- Immutable charter: each run's debrief belongs in that run's dated report. -->
