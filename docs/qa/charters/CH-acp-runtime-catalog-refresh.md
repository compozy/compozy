# CH-acp-runtime-catalog-refresh: Prove live catalogs update without code changes

```yaml
charter:
  id: CH-acp-runtime-catalog-refresh
  mission: "As Ada, prove live provider catalogs discover new models and typed options automatically while preserving stale data, isolation, and structured-surface parity."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-20
  scenarios: [MS-live-model-release-refresh, MS-cursor-account-model-discovery, RT-hermes-live-model-readiness, MS-042, RT-model-catalog-cold-open]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Start from persisted rows, introduce one newly advertised fixture row, and prove TTL, periodic, and explicit refresh publish it without blocking the initial read."
      - "Force one refresh failure and prove old rows remain stale; compare logical IDs, descriptors, configurations, and status across CLI, HTTP, UDS, native tools, and Web."
      - "Use separate profile/workspace execution contexts and prove one live observation or binding never appears in the other."
      - "Exercise Cursor command discovery and Hermes ACP handshake readiness, including redacted failure diagnostics."
    must_avoid:
      - "Do not treat an explicit curated set as a live-discovery allowlist; inspect view=all before declaring a model missing."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report. -->
