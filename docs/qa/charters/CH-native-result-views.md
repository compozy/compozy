# CH-native-result-views: Inspect and migrate native tool results

```yaml
charter:
  id: CH-native-result-views
  mission: Inspect compact state, request full details, and migrate a Loop reference.
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-agent-marketplace-parity
  scenarios: [ET-native-result-views, ET-cli-tool-invoke-structural-handles, RT-session-spawn-wake]
  tour: Feature
  time_box_minutes: 30
  guidance:
    must_try:
      - Discover both schemas and reuse a compact session identity through CLI and API.
      - Create, spawn, inspect and archive sessions in both views.
      - Reject invalid selectors and validate a full-only Loop reference.
    must_avoid:
      - Live provider accounts and unrelated wake-delivery semantics.
```
