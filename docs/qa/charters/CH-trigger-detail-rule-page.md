# CH-trigger-detail-rule-page: Navigate and inspect trigger rules

```yaml
charter:
  id: CH-trigger-detail-rule-page
  mission: "As Bruno, find a trigger in the catalog, open its rule page, understand its When/If/Then behavior and recent runs, inspect its diagnostics, and return without losing navigation context."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-24
  scenarios: [ET-web-jobs-triggers-catalog, ET-web-trigger-detail-rule-page]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Enter through the Triggers navigation item, use catalog search or filters, open one event trigger, and return through its breadcrumb."
      - "Read the trigger sentence, When/If/Then rule, recent run drawers, properties, reliability, identity, and webhook delivery details when present."
      - "Open Inspect, verify diagnostics and the sample envelope, close it with Escape, then refresh and deep-link back to the same trigger."
      - "Toggle one editable trigger off and on, confirming the persisted state after refresh; confirm managed triggers expose the switch but no Edit or Delete action."
      - "Probe browser back/forward, a malformed trigger URL, keyboard-only navigation, 200% zoom, and a 320x800 viewport."
    must_avoid:
      - "Using source code, internal endpoints, or fixture knowledge to decide whether the user-visible result passed."
```
