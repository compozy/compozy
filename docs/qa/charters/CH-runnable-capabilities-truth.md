# CH-runnable-capabilities-truth: Only runnable skills are offered and dead sidecars heal themselves

```yaml
charter:
  id: CH-runnable-capabilities-truth
  mission: "As Ada, prove when.* gates withhold unrunnable skills from the advertised set while listing them inactive-with-reason, and that a dead MCP sidecar stops being hammered, stays diagnosable, and auto-recovers without a restart."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-offer-runnable-capabilities
  scenarios: [ET-skill-activation-gates, RT-mcp-dead-recovery, ET-001, ET-002]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Gate a skill by platform and by requires_tools: absent from the advertised set and agent prompt (measured token drop), listed inactive with the exact unmet gate across CLI/HTTP/UDS/native/Web (ET-001/ET-002 activation views)."
      - "Make the required tool available → next catalog projection activates without restart; unknown when.* key → parse error; no when block → identical to today."
      - "Drive one workspace's sidecar to five confirmed-permanent failures → dead mark, low-frequency probing, unavailable-with-reason; workspace B unaffected; transient timeouts never mark dead."
      - "Repair, wait for the due probe, and confirm auto-clear with normal cadence — no manual revive control anywhere."
    must_avoid:
      - "Conflating administrative enabled state with runtime activation — they must stay independent."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
