# CH-codex-native-tools-cgo0: A Codex session discovers native tools on macOS without CGO

```yaml
charter:
  id: CH-codex-native-tools-cgo0
  mission: "As Ada, start a managed Codex session on the shipped macOS build, discover a compozy__ native tool, invoke it, and confirm the hosted MCP bridge remains bound after a fresh read."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-validate-compozy-hard-cut
  scenarios: [ET-compozy-native-tool-invocation, ET-048]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Start from the isolated runtime and create one native_cli Codex session without changing the operator's provider home."
      - "Ask for a real compozy__ read, then confirm the same session and tool through the operator CLI or public API after the turn settles."
      - "Restart or reconnect the read surface once; native tool projection must remain available without a retry workaround."
    must_avoid:
      - "Do not substitute direct CLI output for the managed-session tool call or expose QA framing in the prompt."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
