# CH-resource-docs-mutation-boundary: Verify resource documentation boundaries

```yaml
charter:
  id: CH-resource-docs-mutation-boundary
  mission: "Follow the desired-state resource guide as Ada and verify that every mutation example uses its owning public lifecycle surface."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-agent-marketplace-parity
  scenarios: [SITE-resource-mutation-boundary]
  tour: Feature Tour
  time_box_minutes: 20
  guidance:
    must_try:
      - "Run the generic optimistic create, update, conflict, invalid-spec, and delete example through the operator UDS."
      - "Attempt the documented bundle lifecycle path and confirm that the guide never asks for direct bundle.activation mutation."
      - "Capture rendered-page or source-generation evidence plus runtime responses."
    must_avoid:
      - "Do not weaken the runtime's service-owned mutation guard."
      - "Do not infer a pass from prose inspection alone."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
