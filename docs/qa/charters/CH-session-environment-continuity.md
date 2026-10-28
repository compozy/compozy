# CH-session-environment-continuity: Continue one conversation across checkouts

```yaml
charter:
  id: CH-session-environment-continuity
  mission: "As Ada, move one idle conversation between ready checkouts and the workspace root while preserving its history, files, identity, and recovery behavior."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-worktree-management
  scenarios: [RT-session-worktree-lifecycle, RT-session-worktree-resume-refusal, RT-worktree-web-session-environment]
  tour: Data Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Use Claude and Codex to remember a phrase and write distinct relative sentinel files in root, checkout A, and checkout B; return to root and verify all three files remain separate."
      - "Bind, change, and unbind through CLI, HTTP, UDS, and native tools; compare fresh session reads with the open Web environment, branch, list, and file picker."
      - "Cancel the environment selector; attempt stale, busy, archived, governed, unavailable-target, and foreign-scope changes and require a typed refusal without partial location changes."
      - "Restart only the isolated lab daemon after an accepted change, then continue the retained conversation in the selected checkout."
      - "Fork a conversation as the adjacent canary and confirm it still creates a separate session."
    must_avoid:
      - "Treating a chip or an agent's claim as filesystem evidence."
      - "Restarting the operator daemon or editing persisted runtime state directly."
  coverage:
    tier: targeted
    surfaces: [CLI, HTTP, UDS, native-tools, ACP, web, filesystem]
    hot_spots: [runtime-release, binding-CAS, recovery, cross-client-cache]
    expected_evidence: "Provider turns, structured session reads, independent sentinel contents, rendered UI, refusal payloads, and restart receipts."
    exit_criteria: "The same conversation runs in the selected directory after each accepted change, existing files stay put, refusals preserve state, and connected clients converge without reload."
```

<!-- The charter is durable and immutable; each run's debrief belongs in its dated report. -->
