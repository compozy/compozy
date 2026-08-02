# CH-extension-kit-lifecycle: Bring one complete extension kit live and retire it cleanly

```yaml
charter:
  id: CH-extension-kit-lifecycle
  mission: "As Bruno, use the documented extension lifecycle to install one inert complete kit, preview it, enable it as one owned unit, inspect its real runtime effects in Web and structured surfaces, then update, disable, and remove it without orphan state."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-kit-lifecycle
  scenarios: [ET-extension-code-first-authoring, ET-ext-kit-enable, ET-ext-inventory, ET-ext-preview, ET-020, ET-021, ET-022, ET-window-manager-hooks-resources, ET-web-extension-detail, ET-web-extension-kit-inventory, ET-web-extensions-manage, ET-web-marketplace-installed-management]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Use the cycle's fresh eng-qa-bootstrap manifest only: non-default COMPOZY_HOME, manifest-derived HTTP/UDS/tmux/provider paths and COMPOZY_WEB_API_PROXY_TARGET, registered long-lived PIDs, and mandatory targeted teardown."
      - "Build and validate a code-first fixture declaring agent directories with SOUL/HEARTBEAT, one skill, loop, tool, MCP server, hook, enabled and disabled automation, and a window layout. Inspect the generated manifest and immutable generation before installation."
      - "Install and immediately compare inventory, resources, scheduler, tool catalog, agent catalog, and Web detail. Every shipped item exists, every live flag is false, and no subprocess or automation began merely because installation succeeded."
      - "Preview through CLI json/jsonl/toon, HTTP, UDS, and native tool; resolve one agent conflict and one missing-env diagnostic; fresh reads before and after each preview must be identical."
      - "Enable with the already accepted current digest, then compare ExtensionEnableResult.automation_started with the extension.enabled event count, scheduler registrations, owner-attributed resources, layout application, inventory, and the browser detail at 375, 768, and 1280 pixels."
      - "Apply a same-requirement kit update, then disable and remove. Bindings survive update, disabled inventory remains shipped-but-not-live, removal clears bindings and owned refs, and fresh resource/automation/tool/agent reads show no orphan state or collateral deletion."
    must_avoid:
      - "Deep secret error permutations (CH-extension-secrets-instance-isolation), digest races or changed-requirement update consent (CH-extension-network-consent), and retired Bundle probes (CH-bundle-product-hard-cut)."
```

## Evidence expectations

- Generated manifest and build receipt; before/install/preview/enable/update/disable/remove structured
  state bundle; automation and owner inventory comparisons; browser captures for kit inventory and the
  shared confirmation affordance.
- Exact bootstrap manifest path, registered PID files, and final `teardown.json` path are recorded in
  the dated execution report.

## Selection rationale

Targeted tier owner for Safety Invariants 1–4, 9–10, and 16 and ADR-002, ADR-004, ADR-006,
ADR-007, and ADR-008.

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
