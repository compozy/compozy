# CH-site-docs-marketplace-truth: Follow the public docs and Marketplace without invented runtime claims

```yaml
charter:
  id: CH-site-docs-marketplace-truth
  mission: "As Dora, run the Feature Tour from the docs landing through representative guides, examples, API reference, and Marketplace routes, proving the rendered information architecture and commands match the shipped runtime while the OpenDesign visual language remains coherent."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-evaluate-compozy-beta
  scenarios: [ET-site-docs-api-reference-ui, ET-site-docs-examples-wave-one, ET-site-docs-first-session, ET-site-docs-masthead-opendesign, ET-site-docs-sidebar-opendesign, ET-site-docs-single-tree-ia, ET-site-docs-typography-opendesign, ET-site-marketplace-bridges-bundled, ET-site-marketplace-catalog]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Enter through /docs, follow the audience path into runtime guides, collapse an active sidebar folder, reload, and confirm the explicit close state persists while Overview remains navigable."
      - "Open prose, example, and dense API pages at desktop and mobile widths; verify hierarchy, actions, copy affordances, OpenAPI operation anatomy, keyboard focus, and direct anchor/deep-link behavior."
      - "Walk Marketplace root, kind list, detail, bridges, and bundled-runtime routes; compare visible inventory with the checked-in catalog and manifests, and confirm all acquisition copy routes through daemon search rather than promising an install from the static site."
      - "Visit removed /runtime and /protocol paths and confirm a real 404 while canonical /docs paths, setup-guide links, and all example commands resolve."
    must_avoid:
      - "Treating prototype fixture copy or placeholder data as normative; runtime truth, COPY.md, and the repository manifests own those axes."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
