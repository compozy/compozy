# CH-memory-operational-state-boundary: Keep runtime chatter out of durable memory

```yaml
charter:
  id: CH-memory-operational-state-boundary
  mission: "As Ada, submit unsafe and safe memory candidates through structured CLI/API surfaces and prove that rejection leaves no file while the adjacent durable write survives a fresh read."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-store-durable-memory-safely
  scenarios: [MS-reject-operational-memory-state]
  tour: Garbage Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Submit qualified native, dotted event, and policy identifiers; inspect structured rejection output and confirm no target appears in a fresh list."
      - "Submit nearby durable content, then confirm its target through a separate list and show read."
      - "Compare CLI and HTTP status/body semantics for one rejected candidate."
    must_avoid:
      - "Starting provider-backed sessions; use only the documented HTTP origin field for production-like collision confirmation."
      - "Starting the Web app; this is a bounded CLI/API data-integrity journey."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
