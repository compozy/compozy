# CH-compozy-agent-authored-review: Deterministic review rounds close only after complete triage

```yaml
charter:
  id: CH-compozy-agent-authored-review
  mission: "As Bruno, run the provider-free review-and-fix loop, inspect deterministic workspace artifacts, reject incomplete finalization, and finish only after a fresh clean agent review."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-08
  scenarios: [LP-agent-authored-review-run, LP-review-artifact-inspection, LP-review-round-finalization]
  tour: Interrupt Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Start review-and-fix with task_name and optional reviewer/fixer/auto_commit inputs through Web, CLI, HTTP, UDS, and compozy__loop_run; confirm there is no PR provider, gh, watch, resolve, or push input."
      - "Return structured issues with and without locations, inspect the exclusive reviews-NNN round and deterministic bytes, then attempt traversal, symlink, and cross-workspace access."
      - "Interrupt a complete fixer batch by leaving one issue pending or malformed; finalization must fail without changing any other issue status."
      - "Complete every triage, prove monotonic resolved/invalid counts, then return an empty reviewer result and verify terminal done without another artifact round."
    must_avoid:
      - "Historical LP-029, CodeRabbit, pull-request watch events, provider provenance, gh shims, or the old CH-005 debrief assumptions."
      - "Creating, renaming, or finalizing issue files from the fixer agent."
  coverage:
    tier: targeted
    surfaces: [Web, CLI, HTTP, UDS, native-tools, run-agent, extension-tools, workspace-filesystem, loop-status]
    invariants: [6, 7, 8, 9, 13]
    adrs: [ADR-003, ADR-008]
    expected_evidence: "Provider-free start parity, byte-stable artifacts, containment failures, atomic finalization counts, and clean second-review termination."
    exit_criteria: "Only trusted Go tools mutate contained artifacts, incomplete rounds never partially finalize, and a fresh empty review is the sole done branch."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->

