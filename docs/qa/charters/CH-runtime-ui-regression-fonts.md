# CH-runtime-ui-regression-fonts: Prove bundled fonts load without weakening CSP

```yaml
charter:
  id: CH-runtime-ui-regression-fonts
  mission: "As Dora, open both production UI doors and prove every bundled font loads under the unchanged strict CSP."
  mode: scenario-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-desktop-shell
  scenarios: [ET-web-font-assets-strict-csp]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Wait for document.fonts.ready in the daemon-served SPA and packaged app, then inspect the first-page console before any reload can hide violations."
      - "Confirm the production policy still says font-src 'self' and that font requests use same-origin WOFF2 URLs rather than data URLs."
      - "Reload each surface once and confirm the result stays clean."
    must_avoid:
      - "Do not relax the CSP or count a development-server page as production bundle evidence."
      - "Do not infer a pass from build output or automated tests alone."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
