# CH-runtime-selector-scroll-boundary: Keep runtime-list scrolling inside its popup

```yaml
charter:
  id: CH-runtime-selector-scroll-boundary
  mission: "As Sol, scroll a long runtime catalog to both ends and keep the session page fixed behind the accessible selector."
  mode: charter-with-tour
  persona:
    name: Sol
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-17
  scenarios: [RT-068]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open the selector from the session composer and use wheel, trackpad-equivalent, keyboard, and Escape."
      - "Reach both list boundaries, continue scrolling, and confirm the product behind the popup never moves."
      - "Select a model afterward and prove the next prompt retains normal runtime selection."
    must_avoid:
      - "Do not settle broader model curation or provider-auth behavior."
```

<!-- Immutable charter: each run's debrief belongs in its dated report. -->
