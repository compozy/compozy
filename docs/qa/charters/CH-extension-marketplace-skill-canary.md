# CH-extension-marketplace-skill-canary: Keep adjacent Marketplace and Skills acquisition healthy

```yaml
charter:
  id: CH-extension-marketplace-skill-canary
  mission: "As Bruno, run a compact Feature Tour through Marketplace and Skills after the extension changes, proving shared catalog navigation, installed-state projections, update badges, and the bundled Compozy skill still work without extension-specific regressions leaking into adjacent capability kinds."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-marketplace-acquisition
  scenarios: [ET-web-marketplace-landing-browse, ET-web-marketplace-search-fanout, ET-web-marketplace-skill-install, ET-compozy-official-skill-discovery, ET-web-extension-union-install]
  tour: Feature Tour
  time_box_minutes: 45
  guidance:
    must_try:
      - "Browse and search Marketplace across Skills and Extensions, install one skill, inspect the bundled compozy skill, and return to the Extensions source-union dialog. Shared counts, installed state, errors, and update badges must stay truthful."
      - "Break only the extension discovery source. Skills remain searchable and installable, the extension section owns its degraded state, and an already installed extension remains manageable."
      - "Refresh and deep-link between one skill detail and one extension detail, then compare the corresponding CLI/HTTP installed projections for stable identities."
    must_avoid:
      - "Executing the full four-kind under-a-minute journey, changing trust policy, or treating the canary as a substitute for the targeted extension charters."
  evidence_expectations:
    - "Marketplace/Skills/Extensions navigation captures, per-kind failure isolation, installed-state reads, and the official skill's canonical identity across Web and structured surfaces."
```

## Selection rationale

Adjacent canary tier. Marketplace, Skills, and Extensions share catalog navigation, search fan-out,
installed projections, and update presentation. This box detects collateral regressions without
diluting any targeted invariant owner.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
