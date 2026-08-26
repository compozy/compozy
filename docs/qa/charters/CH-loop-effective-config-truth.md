# CH-loop-effective-config-truth: Trace effective config and halt safely

```yaml
charter:
  id: CH-loop-effective-config-truth
  mission: "As Bruno and Ada, configure a deterministic Loop, trace why each effective value won, and confirm halt ends automatic work without removing explicit recovery."
  mode: collaborative
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-configure-and-run-loop
  scenarios: [LP-effective-config-provenance, LP-halt-on-node-failure]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Compare inspect, dry-run, admitted status, HTTP, and native-tool effective config sources before and after changing current defaults."
      - "Use explicit zero and false values and confirm their JSON Pointer sources are not lost."
      - "Fail a node under halt, wait for automatic admission, then use one explicit rerun."
    must_avoid:
      - "Reading SQLite as evidence or treating generated contract tests as a public-interface verdict."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
