# CH-git-rebase-helper-safety: Keep rebase diagnostics truthful for real paths

```yaml
charter:
  id: CH-git-rebase-helper-safety
  mission: "As Ada, diagnose and validate a real rebase conflict whose filename contains spaces, then confirm the helpers report the resolved state and repository-owned gate accurately."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-offer-runnable-capabilities
  scenarios: [ET-git-rebase-helper-safety]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Create a real Git conflict in a filename containing spaces and run analyze-conflicts.sh from the public bundled skill path."
      - "Resolve and stage the conflict, then re-run the analyzer and validate-merge.sh."
      - "Independently inspect Git's NUL-delimited unmerged-path output and compare it with the helpers."
      - "Confirm validation recommends make gate-full and repository-root Turbo commands without npm or package-local frontend commands."
    must_avoid:
      - "Do not use internal Go calls, edit Git index files directly, or treat source inspection as behavioral proof."
      - "Do not run the helper against the operator's active repository conflict state."
```

The charter is durable and immutable. Session debriefs belong in dated reports.
