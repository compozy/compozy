# CH-retired-product-catalogs: Use surviving product surfaces and confirm retired product commands, routes, and navigation stay absent after restart.

```yaml
charter:
  id: CH-retired-product-catalogs
  mission: "Use surviving product surfaces and confirm retired product commands, routes, and navigation stay absent after restart."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-validate-compozy-hard-cut
  scenarios: [ET-retired-product-surfaces-absent]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Discover CLI, API, native tools, hooks, and Settings through public interfaces."
      - "Attempt retired commands and routes, then independently read surviving Tasks and Goal surfaces."
      - "Reload Web and restart the isolated daemon; repeat public reads."
    must_avoid:
      - "No external services, paid providers, or mocked daemon behavior."
      - "No direct database queries or changes; preserve unrelated runtimes."
```
