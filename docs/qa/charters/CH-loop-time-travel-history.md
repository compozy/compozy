# CH-loop-time-travel-history: Repair and branch history without rewriting it

```yaml
charter:
  id: CH-loop-time-travel-history
  mission: "As Bruno with the same run open in two tabs, compare, amend, rerun, and fork history while preserving the source and admitting one idempotent operation."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-replay-loop-history
  scenarios: [LP-amend-rerun, LP-time-travel-diff, LP-time-travel-rerun, LP-time-travel-fork, LP-web-amend-rerun-dialogs, LP-web-run-diff-view, LP-web-fork-dialog]
  tour: Multi-Tab Tour
  time_box_minutes: 60
  invariants: [Safety 7 fork snapshot isolation, Safety 8 one time-travel intent, Safety 9 append-only amendment]
  guidance:
    must_try:
      - "Compare CLI, HTTP, UDS, native, and Web diff payloads for generations and linked runs."
      - "Amend one parked output, preserve the original, then rerun the addressed lane and dependents."
      - "Race the same rerun or fork request id from two tabs and reuse it with changed arguments."
      - "Fork with and without input overrides, refresh both runs, and verify seed generation, executing generation, and two-way lineage."
    must_avoid:
      - "Cross-loop comparisons presented as success, direct store inspection, or accepting a source-row mutation as lineage proof."
```

