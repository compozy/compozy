# CH-loop-agent-capabilities: Enforce responder and time-travel authority

```yaml
charter:
  id: CH-loop-agent-capabilities
  mission: "As Ada, discover and operate Loop requests and history through native tools while proving capability grants work and durable self-lineage always fails closed."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-supervise-loop-request
  scenarios: [LP-agent-self-denial]
  tour: Feature Tour
  time_box_minutes: 60
  invariants: [Safety 2 daemon-owned self-operation denial]
  guidance:
    must_try:
      - "Grant and deny loops.respond and loops.timetravel independently, then invoke each gated native tool."
      - "Answer an opted-in unrelated run, then attempt the same operation as the run starter and a spawned descendant."
      - "Attempt rerun and fork on the agent's own executing run and compare the deterministic error across native, HTTP, and UDS surfaces."
      - "Resolve a cross-workspace and stale-lineage target and confirm it fails closed before data is revealed."
    must_avoid:
      - "Operator identity as a substitute for an agent actor or injected actor fields supplied by the client."
```

