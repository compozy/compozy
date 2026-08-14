# CH-worktree-bootstrap-hooks: Configuration and hooks change outcomes without leaving debris

```yaml
charter:
  id: CH-worktree-bootstrap-hooks
  mission: "As Dora, vary worktree configuration and lifecycle hooks across manual, adopted, and per-run paths, then prove explicit policy can deny while execution failures stay fail-open and bootstrap failures remain truthful and contained."
  mode: strategy-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-worktree-management
  scenarios: [MS-worktree-config-bootstrap, ET-worktree-hook-event-contract, TA-task-per-run-worktree-isolation]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Read defaults, apply a parent-workspace overlay, change every [worktrees] key and default_worktree_mode sequentially, and prove live edits affect future operations without moving or reclassifying existing worktrees."
      - "Use invalid roots, namespaces, durations, and empty patterns; each public config write or subsequent operation must name the exact invalid key without partial mutation."
      - "Plant ignored copy files, existing destination files, a slow setup, a failing setup, secret-shaped output, and hostile GIT_* variables; verify no overwrite, process-group timeout, redaction, and a usable ready worktree with setup_state=failed."
      - "Deny pre_create and pre_remove explicitly, make each hook crash once, fail observe consumers, create per_run work, and adopt an existing checkout; only explicit denies block, cleanup is complete, and adoption skips bootstrap/pre_create."
    must_avoid:
      - "Parallel config writes against the shared isolated home."
      - "Real secrets or setup commands that mutate outside the isolated lab."
  coverage:
    tier: targeted
    surfaces: [config.toml, config-CLI, hooks, events, manual-create, adoption, task-per-run]
    invariants: [8, 12, 13, 15, 19]
    hot_spots:
      - "Bootstrap and per-run share one phased creation primitive, so every failure must unwind the same owned artifacts."
      - "Only a named deny may block; hook and observer failures must not corrupt lifecycle transitions."
    adrs: [ADR-006, ADR-009, ADR-011]
    expected_evidence: "Resolved config before/after, setup environment captures, Git/registry orphan scans, hook verdicts, and ordered redacted lifecycle events."
    exit_criteria: "Config applies at the promised lifecycle, bootstrap failures are contained and truthful, explicit denies are deterministic, and all fail-open paths complete without residue."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief belongs in that run's dated report. -->
