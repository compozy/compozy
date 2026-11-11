# CH-otel-export-collector-walk: An operator sees one trace per turn in their own collector, and status tells the truth

```yaml
charter:
  id: CH-otel-export-collector-walk
  mission: "As Dora, enable OTLP export against the _dx.md Collector → Jaeger + Prometheus stack and prove that CLI, web, and desktop prompts become one nested trace per turn, the p95 query works, status agrees across CLI/HTTP/UDS in every export state, and nothing private leaves the machine."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-export-telemetry-to-collector
  scenarios: [OBS-otel-export, RT-001]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Walk OBS-otel-export sections 1-2 first (golden path, then every status state over all four surfaces); they gate the rest."
      - "Capture the traceparent of the web composer's POST …/prompt and look its trace id up in Jaeger before anything else in section 3: code reading says that request bypasses the apiClient middleware."
      - "Forward to a real OTel-capable provider (Claude Code) and confirm its own series carry the session id; read the docs policy statement."
      - "Ask a live agent to change observability.otel.* through compozy__config_set and confirm config_trust_root_forbidden."
    must_avoid:
      - "Do not re-prove hermetic behavior already owned by the Regression hot spots tests (sanitizer field rules, hosted parenting races, write attribution)."
      - "Do not point the lab at a shared or production collector, and never capture a resolved header value."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
