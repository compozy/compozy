# CH-compozy-landing-canary: A first-time reader understands the integrated OS claim

```yaml
charter:
  id: CH-compozy-landing-canary
  mission: "As Cora, judge the locally rendered landing as the cycle's one adjacent canary and decide whether Compozy is an integrated agent OS without relying on unpublished install evidence."
  mode: charter-with-tour
  persona:
    name: Cora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-evaluate-compozy-beta
  scenarios: [REL-os-landing-proof]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Render the landing locally at desktop and mobile widths; read the locked hero definition, inspect the static OS shell, and follow all six ordered sections through the CTA."
      - "Explain in plain language how work, memory, permissions, coordination, and extensibility form one system and identify which proof supports the claim."
      - "Check that comparison claims cite their sources, beta copy is explicit, and no control or metric implies runtime support that does not exist."
    must_avoid:
      - "Installing a beta, calling a live registry, testing Sigstore/cosign output, changing hosting routes, or touching DNS."
      - "Selecting REL-beta-install-paths, REL-beta-installer-provenance, or REL-beta-self-update; they remain post-publish backlog."
  coverage:
    tier: adjacent-canary
    surfaces: [local-landing, responsive-layout, static-OS-shell, sourced-comparison, beta-copy]
    invariants: [10]
    adrs: [ADR-005]
    expected_evidence: "Desktop/mobile browser captures plus Cora's plain-language explanation and unsupported-control sweep."
    exit_criteria: "The local page communicates one integrated OS truthfully; no live publication claim or unsupported runtime control appears."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->

