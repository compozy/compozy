# CH-truthful-cost-provenance: Every dollar shown is real, badged, or absent — never fake

```yaml
charter:
  id: CH-truthful-cost-provenance
  mission: "As Rafa, audit session and task cost across all four provenance states and the five-rate catalog chain, proving estimated never masquerades as actual, included/unknown never show an amount, and a missing active-bucket rate fails closed everywhere."
  mode: charter-with-tour
  persona:
    name: Rafa
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-14
  scenarios: [RT-session-cost-provenance, TA-task-run-cost-provenance, ET-model-source-five-rate-pricing, MS-042, MS-045, MS-055, MS-056, ET-053]
  tour: Money Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "One finished session per state: actual/agent_reported, estimated (badge + source visible, full-width row per ADR-012), included (no $), unknown (no amount, no error) — Web inspector, CLI -o json, and HTTP must agree on reload."
      - "Five-bucket estimate with all rates, then remove one active-bucket rate → unknown/none, no substitution; task roll-up with incompatible child provenance fails closed amountless."
      - "Five-rate chain regression sweep: extension model.source round-trip (ET-053/ET-model-source-five-rate-pricing), catalog views and OpenAI projection (MS-042/MS-045), CLI/HTTP/UDS/native parity (MS-055), config round-trip with finite non-negative validation (MS-056)."
      - "Subscription-auth fixture classified included off auth mode — never a hardcoded provider list; cite the recorded account-usage determination for the absent fetcher."
    must_avoid:
      - "Accepting a plain $ anywhere for estimated/included/unknown — provenance must be visible at the point of use."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
