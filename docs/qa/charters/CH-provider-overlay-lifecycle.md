# CH-provider-overlay-lifecycle: Use an account-scoped model catalog

```yaml
charter:
  id: CH-provider-overlay-lifecycle
  mission: "Use a separately named provider runtime, curate its discovered models, and retire its cached catalog safely."
  mode: charter-with-tour
  persona:
    name: Sol
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-17
  scenarios: [RT-model-catalog-cold-open]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Discover and curate an overlay through public CLI and HTTP surfaces, then bind a session."
      - "Remove and recreate the overlay while discovery is unavailable; inspect catalog freshness and ownership."
      - "Inspect Cursor overlay selection and reject an unadvertised model without persisting it."
    must_avoid:
      - "Changing the operator's daemon or login state, or treating fixture catalogs as vendor account evidence."
```
