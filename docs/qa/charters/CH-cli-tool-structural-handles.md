# CH-cli-tool-structural-handles: Generic CLI tool output remains reusable and safe

```yaml
charter:
  id: CH-cli-tool-structural-handles
  mission: "As Ada, prove generic CLI tool invocation preserves every daemon-authored public handle needed by the next operation while continuing to redact actual secrets."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-agent-marketplace-parity
  scenarios: [ET-cli-tool-invoke-structural-handles]
  tour: Feature Tour
  time_box_minutes: 20
  guidance:
    must_try:
      - "Invoke a real native tool that returns high-entropy public IDs, digests, and a continuation cursor; compare complete CLI JSON with HTTP and UDS for the same workspace and daemon state."
      - "Reuse each returned structural handle in its next supported operation instead of checking only its shape."
      - "Include sensitive-key fields and secret-shaped free text and prove they remain redacted without erasing neighboring public handles."
    must_avoid:
      - "Disabling redaction, bypassing the generic `compozy tool invoke` path, or substituting snapshots for live structured output."
  evidence_expectations:
    - "Exact cross-plane JSON comparisons, successful handle reuse, redaction assertions, two-workspace isolation, and cleanup proof."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
