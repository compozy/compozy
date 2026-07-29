# CH-workspace-binding-canary: The ordinary same-workspace path still resolves and still binds

```yaml
charter:
  id: CH-workspace-binding-canary
  mission: "As Ada, run the everyday same-workspace path — resolve context from the precedence chain and operate bound native surfaces without naming a workspace — and prove the new cross-workspace policy changed nothing about work that never crosses."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-workspace-context
  scenarios: [ET-native-workspace-scope-isolation, MS-workspace-resolution-chain]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Bootstrap a fresh isolated lab with unique COMPOZY_HOME/ports/provider home/tmux socket, register PIDs, and run eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every pass/fail/blocked/abort exit; cite clean teardown.json."
      - "Register a root, a nested root inside it, and a sibling root; from subdirectories of each, run workspace info, Loop, config, memory, and session creation with no --workspace and confirm the nearest enclosing root wins and no subdirectory registration is minted."
      - "Walk the precedence chain once end to end — positional over flag, flag over environment, environment over validated session identity, identity over cwd — and confirm resolution_source names the winning tier in structured output."
      - "From a workspace-bound session, invoke representative native reads and mutations with no workspace input and confirm dispatch fills the bound workspace, the handler receives it, and pre-call hooks cannot rewrite it."
      - "Supply the session's own workspace explicitly by id, name, and path and confirm each canonicalizes to the same registry id and proceeds as an ordinary same-workspace call, with no policy prompt and no denial."
      - "From an unregistered directory, confirm the shared parseable error still names the tried tiers and the registration fix."
    must_avoid:
      - "Every cross-workspace outcome, prompt, and consent behavior — this is the canary for what did not change; CH-cross-workspace-mode-seams and CH-cross-workspace-consent-audit own the new behavior."
      - "Any web surface."
      - "Parallel config writes against the shared isolated home."
  coverage:
    tier: targeted
    surfaces: [CLI, native-tools, HTTP, UDS, session-identity, workspace-resolver]
    invariants: [1, 3]
    hot_spots:
      - "Invariant 1's regression floor: the existing isolation and resolution suites were expected to survive the seam wiring with edits only — this session is the human-visible half of that claim."
      - "Invariant 3 canonical ids: raw refs must normalize to the registry id before any policy or handler sees them, including on the same-workspace path where no policy branch fires."
    adrs: [ADR-001, ADR-007]
    expected_evidence: "Resolution matrices with resolution_source per tier, the nested-root catalog before and after, and native call traces showing the bound workspace reaching the handler with no prompt and no denial."
    exit_criteria: "Every same-workspace operation resolves and lands exactly as it did before the program, no nested registration appeared, and no ordinary call triggered the cross-workspace policy path."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief belongs in that run's dated report. -->
