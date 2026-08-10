# CH-desktop-links-instance-macos: Deep links, single instance, and link boundaries on macOS (scripted-manual smoke)

```yaml
charter:
  id: CH-desktop-links-instance-macos
  mission: "As Théo on macOS, throw valid, dead, hostile, and rapid compozyos:// links at the app — running, cold, and mid-provisioning — and prove one window, the right view, and a hard external-link boundary; scripted-manual with recorded evidence (macOS has no WebDriver and no IT-020 runner, so E2E-007/E2E-023 are the macOS single-instance proof)."
  mode: charter-with-tour
  platform: macos
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
      - "E2E-008: with the app running, `open compozyos://open/<existing-session-path>` → focus + correct view; a deleted-entity link → product not-found view; a malformed/hostile payload (`compozyos://open/http://evil.com`, traversal) → default view, no dialog."
      - "E2E-023: app closed → activate a link into a home needing start/provision → cold start runs the states and the linked view renders once ready; exactly one navigation."
      - "E2E-007: launch again from dock/Launchpad/`compozy app open` → existing window focused/unminimized, `ps` count unchanged; repeat with a link argument → forwarded, never dropped."
      - "E2E-009: external https link inside the product → default browser opens, the app stays on the product; `target=_blank` behaves identically."
    must_avoid:
      - "Never accept an off-product navigation or a rendered foreign origin as anything but a Blocks-Completion finding."
      - "Never run against the operator's default home; never leave lab processes alive past teardown."
```

<!-- Immutable charter: each run's debrief belongs in that run's dated report. -->
