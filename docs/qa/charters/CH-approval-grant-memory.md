# CH-approval-grant-memory: An always answer survives restart and stays revocable

```yaml
charter:
  id: CH-approval-grant-memory
  mission: "As Théo, answer native-tool prompts with allow_always/reject_always, restart the daemon between every proof, and verify grants persist at the most-specific key, wider grants exist only via explicit set, revocation restores prompting, and deny-all always wins."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-answer-agent-requests
  scenarios: [ET-native-tool-approval-grants]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "allow_always then an identical call (zero prompts), restart, repeat; reject_always then the deterministic auto-deny; allow_once persisting nothing."
      - "Explicit agent-wide and tool-wide set through native, CLI, HTTP, UDS, AND Web; verify the persisted keys carry no input_digest and list identically on every surface after restart."
      - "Cross checks: workspace-B never matches, a non-matching agent still prompts, deny-all denies despite a stored allow, and a grant-store read error falls to the prompt."
      - "Revoke each row through a different surface than the one that created it; the next matching call must prompt."
    must_avoid:
      - "Conflating this plane with ACP subprocess fs-approvals or sandbox PermissionDecisionAllowAlways — both are out of scope and must stay unchanged."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
