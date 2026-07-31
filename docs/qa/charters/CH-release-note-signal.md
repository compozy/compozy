# CH-release-note-signal: Review product-only release notes

```yaml
charter:
  id: CH-release-note-signal
  mission: "As Dora, render the next beta candidate and inspect the public release history to confirm product changes remain visible while repository-maintenance commits stay out of release notes."
  mode: scenario-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-approve-compozy-beta-candidate
  scenarios: [REL-release-note-signal]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Render the next beta body with the pinned release planner and confirm feature, fix, refactor, breaking, and authored release-note content remains."
      - "Inspect the open release PR and every published GitHub Release through public GitHub interfaces for docs, build, or CI conventional-commit entries."
      - "Re-open the public bodies through an independent GitHub read path and confirm the filtered state survives."
      - "Stop candidate approval if any maintenance entry remains or any product-facing breaking change disappears."
    must_avoid:
      - "Creating, moving, or pushing a tag, publishing a release, or merging the release PR."
      - "Treating repository source or an internal test helper as proof of the public GitHub state."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
