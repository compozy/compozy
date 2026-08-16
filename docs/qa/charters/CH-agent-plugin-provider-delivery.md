# CH-agent-plugin-provider-delivery: Consume one portable package through all three providers

```yaml
charter:
  id: CH-agent-plugin-provider-delivery
  mission: "As Ada, use real Claude Code, OpenClaw, and Hermes sessions to prove one daemon-ingested package delivers the same skill, stdio MCP environment, and daemon-hosted remote MCP behavior to every provider."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-distribution
  scenarios: [ET-agent-plugin-provider-delivery, ET-agent-plugin-conformance-walk]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Install and enable one generation once, then create one real managed session per provider without provider-specific package installation or pass-through."
      - "In every session activate the same ingested skill, call the stdio fixture, prove absolute PLUGIN_ROOT and writable PLUGIN_DATA, and call the same streamable-http server through the daemon."
      - "Record provider, session id, first-attempt result, skill observable, stdio env/data observable, remote observable, redaction scan, and evidence paths in provider-matrix.json."
      - "Use provider homes exactly as the bootstrap manifest declares; preserve operator HOME only for native_cli plus home_policy=operator."
    must_avoid:
      - "Replacing a real provider with acpmock, counting a retry while hiding the first failure, projecting remote credentials or plugin format into provider config, or using separate installed generations per provider."
```

## Evidence gate

`docs/qa/evidence/<run-date>-agent-plugins/provider-matrix.json` must contain three completed rows
named `claude-code`, `openclaw`, and `hermes`. The dated report cites each row and the shared installed
generation. This file and the conformance checklist are the two evidence sources task 09 may cite.

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
