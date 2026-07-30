# CH-provider-settings-model-delta: Persist only explicit model overrides

```yaml
charter:
  id: CH-provider-settings-model-delta
  mission: "As Ada, round-trip Provider Settings model overrides through HTTP and UDS, restart the daemon, and prove validation failures preserve the prior configuration."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-20
  scenarios: [MS-provider-settings-model-delta-roundtrip]
  tour: Garbage Tour
  time_box_minutes: 45
  guidance:
    must_try:
      - "PUT unchanged curated membership with explicit five-rate and reasoning deltas, then compare config and all-view catalog reads before and after restart."
      - "Submit negative and non-finite rates through both transports and compare typed status, response body, and unchanged config bytes."
      - "Verify catalog enrichment that the operator did not override never materializes in config."
    must_avoid:
      - "Accepting a successful response without restart readback or using generated catalog enrichment as an authored setting."
```
