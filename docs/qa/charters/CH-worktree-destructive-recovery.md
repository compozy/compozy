# CH-worktree-destructive-recovery: Destructive paths preserve every fact they do not own

```yaml
charter:
  id: CH-worktree-destructive-recovery
  mission: "As Bruno, interrupt exit and removal at their dangerous seams, remove a checkout out of band, and prove recovery never guesses clean state, deletes history, or removes a branch it cannot compare safely."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: flaky
    locale: en-US
  journey: J-worktree-management
  scenarios: [RT-worktree-exit-commit-scope, RT-worktree-exit-push-publish, RT-worktree-exit-pr-idempotency, RT-worktree-exit-merged-cleanup, RT-worktree-web-exit-ladder, RT-worktree-web-exit-progress, RT-worktree-web-merged-cleanup, RT-worktree-web-removal-two-step, RT-worktree-web-missing-resolution, RT-worktree-reconcile-branch-safety]
  tour: Interrupt Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Interrupt commit, push, request, cancel, daemon restart, and removal at phase boundaries; require one durable active operation, completed-step retention, exact-op cancellation, and explicit failed recovery."
      - "Inject status read failure, behind/diverged state, dirty tracked and named untracked files, unique commits, a remote copy, and a running session; every blocker must remain fail-closed and quantified before force is offered."
      - "Remove a checkout outside Compozy, replace its path with a foreign repository, restore the original identity, and dismiss it; sessions, tasks, events, tombstones, and branches must never cascade."
      - "Exercise run branches with unchanged, moved, non-run-namespace, adopted, and operator-created refs; only the recorded Compozy-created unchanged ref may emit branch_reclaimed and disappear."
    must_avoid:
      - "Force-removing any path before capturing its gitdir identity and complete refusal inventory."
      - "Treating absent or stale status and forge evidence as clean."
  coverage:
    tier: targeted
    surfaces: [CLI, HTTP, UDS, web-S6-S14-S15, Git, SQLite, SSE]
    invariants: [3, 4, 5, 6, 9, 10, 11]
    hot_spots:
      - "The ready-to-removing fence, repository lock, under-lock reread, and durable exit index close different races and must compose."
      - "Reconcile-never-cascade, branch preservation, and compare-and-delete are the program's primary data-loss boundary."
    adrs: [ADR-006, ADR-007, ADR-011]
    expected_evidence: "Before/after Git refs and worktree lists, durable operation/event rows, blocker payloads, restart traces, and preserved session/task histories."
    exit_criteria: "Every unknown or unsafe state blocks, interruption recovers truthfully, only one eligible run ref is reclaimed, and all non-owned history survives."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief belongs in that run's dated report. -->
