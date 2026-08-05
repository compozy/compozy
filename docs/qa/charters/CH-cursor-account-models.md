# CH-cursor-account-models: Discover Cursor models before a session exists

```yaml
charter:
  id: CH-cursor-account-models
  mission: "As Ada, discover the signed-in Cursor account catalog before any session and prove its exact IDs, cached status, explicit refresh, and CLI/HTTP/UDS parity."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-20
  scenarios: [MS-cursor-account-model-discovery, MS-042, MS-055]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Start from a fresh isolated Compozy home with the operator's real Cursor native login and no Cursor source status."
      - "List Cursor models through CLI, HTTP, and UDS before creating a session; compare exact IDs including auto and composer-2.5."
      - "Read again without refresh, inspect status, then run one explicit refresh and prove the lifecycle from public surfaces."
    must_avoid:
      - "Do not curate rows or treat curated metadata as a list of permitted runtime IDs."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report. -->
