# CH-worktree-fanout-exit-removal: Parallel runs reach safe cleanup without crossing

```yaml
charter:
  id: CH-worktree-fanout-exit-removal
  mission: "As Bruno, drive a real per-run fan-out from the Web through parallel isolated checkouts, finish one through assisted exit and removal, interrupt another, and prove every sibling keeps its own durable state."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-isolated-task-loop-execution
  scenarios: [TA-task-per-run-worktree-isolation, TA-task-fanout-worktree-isolation, TA-worktree-web-task-policy, TA-worktree-web-fanout-isolation, LP-loop-environment-resolution, LP-worktree-web-loop-environment, RT-worktree-web-exit-ladder, RT-worktree-web-exit-commit-pr, RT-worktree-web-exit-progress, RT-worktree-web-removal-two-step]
  tour: Multi-Tab Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Use the real browser flow to set per_run, enqueue a designated fan-out, and watch every accepted run reach a distinct run branch, worktree, session, status, and final attribution; omission of the idempotency key must send no request."
      - "After enqueue, change the profile and race fresh reads in another tab; queued runs must keep their snapshots, active controls must lock, and no result row may borrow a sibling workspace or worktree."
      - "Take one completed branch through commit, publish or browser fallback, request creation when available, progress replay, cleanup evidence, and removal; keep another running and prove its exit/removal stays blocked without blocking the first."
      - "Run the Loop mode matrix for root, worktree, per_run, and contained directory, including one fan-out node instance and a legacy params.cwd definition that must fail before session start."
    must_avoid:
      - "Using mocked browser data or a general task run list as proof of accepted fan-out result updates."
      - "Assuming a failed POST represents isolated sibling failure; provoke failure after accepted runs exist."
  coverage:
    tier: targeted
    surfaces: [web-S10-S11, web-S14-S15, task-CLI, HTTP, UDS, native-tools, task-stream, Loop-runtime, worktree-SSE]
    invariants: [3, 4, 5, 6, 7, 8, 17]
    hot_spots:
      - "Enqueue snapshot and lease authority must prevent profile races, stale claimants, session reuse, and rollback residue."
      - "Parallel exit and removal fences must isolate siblings while progress remains replayable after interruption."
    adrs: [ADR-006, ADR-009, ADR-011]
    expected_evidence: "Browser captures for each run row and exit phase, structured run/worktree/session payloads, event timelines, Git refs, and orphan scans."
    exit_criteria: "Each accepted run remains uniquely attributable, one sibling failure stays local, the completed checkout exits and removes safely, and no orphan or cross-workspace state remains."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief belongs in that run's dated report. -->
