# CH-site-docs-search-context: Find the right page among duplicate titles

```yaml
charter:
  id: CH-site-docs-search-context
  mission: "As Dora, search the public documentation for duplicate page titles and choose the correct Runtime, CLI, API, or Protocol result from its visible section context."
  mode: scenario-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-evaluate-compozy-beta
  scenarios: [ET-site-docs-search-context]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open search by pointer and keyboard, query Sessions, and inspect every duplicate-title result before opening one."
      - "Open representative Runtime, CLI, API, and Protocol results and verify the breadcrumb predicts the destination."
      - "Repeat after closing and reopening search to catch stale result context or keyboard-focus loss."
    must_avoid:
      - "Using route source or the search index payload as a substitute for rendered keyboard and visual evidence."
```
