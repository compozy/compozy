# CH-cross-workspace-mode-seams: One permission mode, five seams, the same answer everywhere

```yaml
charter:
  id: CH-cross-workspace-mode-seams
  mission: "As Ada, cross into a second workspace from every supported agent surface under each permission mode and prove the outcome, the denial hint, the reason code, and the exit code are the same decision wearing different clothes."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-cross-workspace-access
  scenarios: [ET-workspace-access-mode-matrix]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Bootstrap a fresh isolated lab with unique COMPOZY_HOME/ports/provider home/tmux socket, register PIDs, and run eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every pass/fail/blocked/abort exit; cite clean teardown.json."
      - "Register two workspaces in that one home, then from an approve-all session name the second one on a native tool call, a task claim, a spawn, and a coordination read — each must cross with no prompt and behave in the target exactly as it would at home."
      - "Repeat every crossing under deny-all and confirm no prompt appears at any seam, the denial carries the exact permission-mode hint, native denials report workspace_access_denied, and the agent CLI verbs exit 77 from a daemon-origin denial rather than a local pre-flight block."
      - "Repeat under approve-reads and confirm the identity, task, spawn, and coordination seams deny with the same hint and never prompt — the native-tool prompt itself belongs to CH-cross-workspace-consent-audit."
      - "Run the same crossing over CLI, HTTP, and UDS for the identity, spawn, claim, and coordination routes and diff the error shapes; then confirm the operator path still reaches both workspaces."
      - "Read every changed public guide against what you just observed: CLI spawn; agent spawning; safe-spawn; configuration; event catalog; permissions; workspace index and resolver; plus the official skill's native-tool and agent-definition references. Record any place the guidance overstates or understates the shipped behavior."
    must_avoid:
      - "Answering a native-tool prompt or exercising session-consent reuse and expiry — that is CH-cross-workspace-consent-audit's box."
      - "Any web surface, including the deep-link confirmation."
      - "Re-deriving the policy precedence chain as if it were undecided; the unit suite owns precedence, this session owns the public outcome."
      - "Parallel config writes against the shared isolated home."
  coverage:
    tier: targeted
    surfaces: [native-tools, agent-CLI, HTTP, UDS, task-claim, spawn, network-coordination, event-store, site-docs, official-skill]
    invariants: [1, 7]
    hot_spots:
      - "Invariant 1 named deltas: approve-all crosses freely and is the built-in default; the tool-seam reason moved from ReasonScopeMismatch to ReasonWorkspaceAccessDenied; the denial copy gained the mode hint; the CLI pre-flight block became a daemon-origin exit 77."
      - "Invariant 7: deny-all must never prompt on the workspace axis, at any seam, including the tool seam — the per-axis asymmetry against its ask-everything tool meaning is exactly where a wrong implementation hides."
    adrs: [ADR-007, ADR-001, ADR-006]
    expected_evidence: "A mode-by-seam outcome matrix with the exact hint text, reason code, and exit code captured per surface; CLI/HTTP/UDS error-shape diffs; audit event payloads naming target, seam, source, and mode; and cited doc lines for each guidance check."
    exit_criteria: "Every seam resolves to the outcome its mode promises on every listed surface, deny-all raised zero prompts anywhere, and no shipped guidance contradicts an observed outcome."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief belongs in that run's dated report. -->
