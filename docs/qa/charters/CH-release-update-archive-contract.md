# CH-release-update-archive-contract: Validate a published archive before release

```yaml
charter:
  id: CH-release-update-archive-contract
  mission: "As Dora, run the release archive hook against the real beta.13 artifact and confirm the same policy that accepts runtime extraction gates publication."
  mode: scenario-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-publish-compozy-beta
  scenarios: [REL-release-archive-update-contract]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Download a real Darwin or Linux beta.13 archive and invoke the exact GoReleaser `updateArchiveCheck` command against it."
      - "Record the compressed archive size and extracted `compozy` size beside the hook verdict."
      - "Confirm the GoReleaser configuration calls the hook for `compozy-archive` artifacts before publication."
    must_avoid:
      - "Do not publish, tag, or alter a GitHub release during this charter."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
