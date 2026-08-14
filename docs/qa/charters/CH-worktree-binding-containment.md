# CH-worktree-binding-containment: A session never escapes or forgets its checkout

```yaml
charter:
  id: CH-worktree-binding-containment
  mission: "As Théo, start, filter, fork, spawn from, stop, and resume worktree-bound sessions while trying to make every cwd and child path escape or fall back to the parent checkout."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-worktree-management
  scenarios: [RT-session-worktree-lifecycle, RT-session-worktree-resume-refusal, RT-session-worktree-fork, RT-worktree-web-session-environment, RT-worktree-web-composer-binding-fork]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Create sessions with --worktree and --new-worktree, HTTP/UDS, and compozy__session_create, then compare filtered CLI/API/native reads for the exact persisted binding."
      - "Attempt parent, sibling, symlinked, relative, and hook-rewritten cwd values through ACP launch, sandbox restart, local tools, direct prompts, and inherited child spawn; every gate must resolve the same ready worktree root."
      - "Fork an idle live session to ready and newly created targets, cancel once, repeat once, and invoke mid-turn; prove the original session and files never change and each confirmation creates at most one fresh session."
      - "Remove the checkout out of band, then resume and spawn; require the named missing refusal, preserved transcript, and zero root fallback."
    must_avoid:
      - "Treating a UI chip as binding evidence without a fresh structured session read and a real file-boundary probe."
      - "Changing a live session's binding; the only supported move is a fresh fork."
  coverage:
    tier: targeted
    surfaces: [session-CLI, HTTP, UDS, native-tools, ACP, sandbox, local-tool-host, web-S7-S9-S16]
    invariants: [1, 2, 7, 20]
    hot_spots:
      - "The four cwd gates, child inheritance, and reuse fingerprint must agree on one immutable binding."
      - "Pending, failed, removing, missing, and removed targets must refuse before launch and never become workspace root."
    adrs: [ADR-005, ADR-007]
    expected_evidence: "Session payload diffs, filesystem probes from parent/sibling/worktree roots, fork counts, and missing-resume refusal captures."
    exit_criteria: "Every process and child stays inside the bound ready checkout, every filter agrees, and no invalid binding launches or resumes."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief belongs in that run's dated report. -->
