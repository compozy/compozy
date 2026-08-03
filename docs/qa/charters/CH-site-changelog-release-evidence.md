# CH-site-changelog-release-evidence: Read one complete release from discovery to evidence

```yaml
charter:
  id: CH-site-changelog-release-evidence
  mission: "As Bruno, use the public changelog to understand the latest CompozyOS release and trace its claims back to pull requests, contributors, and downloadable artifacts."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-evaluate-compozy-beta
  scenarios: [SITE-changelog-release-receipts]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Enter through the changelog index, open the latest exact version, and compare its complete notes and categories with the linked GitHub Release."
      - "Follow a linked pull request and contributor, inspect release downloads, refresh the deep link, and open the RSS feed."
      - "Find the latest version through site search and re-read the index and detail page at a 320-pixel viewport with keyboard navigation."
    must_avoid:
      - "Using repository source, internal API responses, or generated test fixtures as substitutes for public browser evidence."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
