# CH-worktree-lifecycle-surface-parity: One worktree lifecycle tells the same truth everywhere

```yaml
charter:
  id: CH-worktree-lifecycle-surface-parity
  mission: "As Ada, create and adopt worktrees through every supported operator and agent surface, then prove identity, state, refusal, approval, and replay stay workspace-scoped and byte-equivalent."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-worktree-management
  scenarios: [RT-worktree-cli-lifecycle, RT-worktree-api-surface-parity, RT-worktree-web-create-adopt, RT-worktree-web-nested-navigation]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Create, cancel, retry, discover, adopt twice, inspect, refresh, and dismiss through structured CLI, HTTP, UDS, and the Web; compare canonical ids, states, diagnostics, bodies, and exit codes."
      - "Invoke all four compozy__worktree tools, confirm their read/mutating/destructive risk metadata and approval behavior, and prove unsupported lifecycle actions remain available through CLI or HTTP/UDS without invented native tools."
      - "Subscribe to the per-worktree and catalog streams, reconnect by after_sequence and Last-Event-ID, and prove ordered, deduplicated, workspace-attributed replay invalidates only the owning workspace cache."
      - "Repeat a read and mutation with another workspace's id and require worktree_not_found with no existence, path, branch, event, or cache leak."
    must_avoid:
      - "Using database rows as the result source; compare only public reads and captured streams, with storage inspection as supporting evidence."
      - "Parallel config writes against the isolated home."
  coverage:
    tier: targeted
    surfaces: [CLI, HTTP, UDS, native-tools, web-S1-S5, SSE, cache]
    invariants: [12, 13, 14, 15, 18]
    hot_spots:
      - "Adoption and creation retries must never mint duplicate runtime identities or mutate a refused directory."
      - "Every read, mutation, stream, and cache path must predicate by workspace and worktree together."
    adrs: [ADR-001, ADR-002, ADR-007]
    adjacent_canary: CH-add-workspace-from-root / J-add-workspace-by-browsing
    expected_evidence: "A surface-by-operation matrix, stream replay captures, native descriptor output, and before/after workspace catalogs."
    exit_criteria: "Every surface reports one scoped identity and state for each operation, retry is idempotent, and cross-workspace probes reveal nothing."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief belongs in that run's dated report. -->
