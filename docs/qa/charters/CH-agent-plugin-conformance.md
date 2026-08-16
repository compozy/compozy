# CH-agent-plugin-conformance: Prove the portable contract before making an external claim

## Cycle preamble

This targeted cycle covers the Agent Plugins diff plus one adjacent native-extension canary. Task 08
must create a fresh lab with `eng-qa-bootstrap`; the resulting `bootstrap-manifest.json` is the sole
authority for `COMPOZY_HOME`, daemon HTTP/UDS addresses, `COMPOZY_WEB_API_PROXY_TARGET`, provider
homes, QA output path, and teardown. The lab uses a run-unique daemon port and tmux socket name; no
default home, port, or tmux server is allowed. Config writes run sequentially within that home. Every
long-lived daemon, web server, provider, browser, watcher, or tmux process registers a pid beneath
`<QA_OUTPUT_PATH>/qa/pids/`. Every exit path evaluates the manifest's `TEARDOWN_COMMAND`, and the run
report cites `teardown.json` with `"clean": true`. Native-CLI providers with operator home policy keep
the operator login; bound-secret or brokered providers use the manifest's isolated provider home.

### Regression hot spots

| Source | Failure target | Owning charter(s) |
|---|---|---|
| Safety 1 | Symlink-resolved package and data paths cannot escape their domains | CH-agent-plugin-conformance; CH-agent-plugin-dev-isolation |
| Safety 2 | Single-pass placeholders affect only args, env values, and cwd | CH-agent-plugin-conformance; CH-agent-plugin-provider-delivery |
| Safety 3 | Package env cannot override PLUGIN_ROOT or PLUGIN_DATA | CH-agent-plugin-conformance; CH-agent-plugin-provider-delivery |
| Safety 4 | Host env is never interpolated; unknown placeholders stay literal | CH-agent-plugin-conformance; CH-agent-plugin-provider-delivery |
| Safety 5 | Package-sensitive headers reject exactly one server | CH-agent-plugin-conformance; CH-agent-plugin-remote-secrets |
| Safety 6 | Secret references resolve only at daemon request construction and never leak | CH-agent-plugin-remote-secrets; CH-agent-plugin-provider-delivery |
| Safety 7 | One MCP policy owns forbidden, OAuth, and case-insensitive duplicate rules | CH-agent-plugin-conformance; CH-agent-plugin-remote-secrets |
| Safety 8 | Adapted instances have zero provides, permissions, and extension subprocess | CH-agent-plugin-conformance; CH-agent-plugin-provider-delivery |
| Safety 9 | Native-root precedence is fixed and invalid selected roots never fall back | CH-agent-plugin-conformance; CH-agent-plugin-native-canary |
| Safety 10 | Every mutation uses one instance coordinator and fixed commit order | CH-agent-plugin-lifecycle-recovery; CH-agent-plugin-dev-isolation; CH-agent-plugin-marketplace |
| Safety 11 | Data survives update; remove deletes or quarantines; double failure aborts | CH-agent-plugin-lifecycle-recovery; CH-agent-plugin-dev-isolation |
| Safety 12 | Fatal manifest errors reject all; component errors skip only that sibling | CH-agent-plugin-conformance; CH-agent-plugin-diagnostics-parity |
| ADR-001 | Portable bytes remain one resource-only extension across all providers | CH-agent-plugin-provider-delivery; CH-agent-plugin-native-canary |
| ADR-002 | Curated feeds use one Extension shelf and a display-only format marker | CH-agent-plugin-marketplace |
| ADR-003 | Install, validate, status, inventory, Web, and agent reads expose diagnostics | CH-agent-plugin-diagnostics-parity; CH-agent-plugin-conformance |
| ADR-004 | Dotted package identity is preserved on every lifecycle plane | CH-agent-plugin-lifecycle-recovery; CH-agent-plugin-provider-delivery |
| ADR-005 | plugin.json stays the only on-disk manifest; reload synthesis is deterministic | CH-agent-plugin-conformance; CH-agent-plugin-dev-isolation |
| ADR-006 | Fixed and secret MCP headers are daemon-owned and source-aware | CH-agent-plugin-remote-secrets; CH-agent-plugin-provider-delivery |

Known-bug watch: `BUG-20260729-mcp-cli-json-parity` remains open and directly intersects the task 08
CLI/HTTP/UDS comparison. Reproduction must update that bug, not mint a duplicate.

```yaml
charter:
  id: CH-agent-plugin-conformance
  mission: "As Ada, prove every Agent Plugins 1.0.0 minimum-conformance item against the stamped build and capture item-level evidence suitable for the compatible-clients claim."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-agent-authoring
  scenarios: [ET-agent-plugin-validation, ET-agent-plugin-native-precedence, ET-agent-plugin-conformance-walk]
  tour: Garbage Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Record the plugin and MCP schema IDs once at the top of docs/qa/evidence/<run-date>-agent-plugins/conformance-checklist.json, then record eight numbered item/status/observable/evidence rows."
      - "Validate conformant, warning-only, fatal, dual-manifest, client-specific, unknown-extension, symlink-escape, reserved-env, sensitive-header, and case-duplicate fixtures without mutating daemon state."
      - "Prove both stdio and streamable-http plus skills; verify fixed-location discovery, single-token command handling, plugin-root default cwd, single-pass expansion, literal unknown placeholders, and daemon-owned root/data env."
      - "Compare human, json, jsonl, toon, and native-tool validation results; fatal codes and ordered component warnings must preserve one semantic contract."
    must_avoid:
      - "Treating automated unit assertions alone as conformance evidence, validating client-specific layouts as portable packages, or summarizing several checklist items into one untraceable pass."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
