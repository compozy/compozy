# CH-extension-dev-link-invoke: Link and invoke one workspace extension through public commands

```yaml
charter:
  id: CH-extension-dev-link-invoke
  mission: "As Bruno, dev-link one locally built extension and prove list, invoke, reload, logs, and remove all resolve the same workspace identity without trust prompts or cross-workspace leakage."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-dev-lifecycle
  scenarios: [ET-extension-dev-reload-loop]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Build and validate one local extension, link it with `compozy extension dev`, and independently list the workspace overlay before invoking its declared tool."
      - "Change the behavior, reload, invoke again, and read logs through CLI so every operation proves the same workspace-scoped instance."
      - "Remove the dev link and fresh-list the workspace to prove the overlay disappears without mutating a global install."
    must_avoid:
      - "Marketplace publishing, public-SDK availability claims, direct database reads, or a repository-internal call that bypasses CLI/HTTP/UDS."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in the dated report (Session Debriefs), never here. -->

