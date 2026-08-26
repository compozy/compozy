# CH-terminal-approval-ladder: Try to make an agent run or type without being asked

```yaml
charter:
  id: CH-terminal-approval-ladder
  mission: "As Bruno, walk every rung of the terminal approval ladder and then attack it — allowlist aggressively, disguise commands, reuse grants across terminals — trying to get a command run or a keystroke typed that nobody approved."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-supervise-agent-terminal
  scenarios: [ET-terminal-approval-ladder-grants]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Approve, then reject, an ordinary agent command; confirm the prompt names the exact command and working directory, and that the rejection reaches the agent as a rejection rather than a failure."
      - "Allowlist a trusted shape and confirm it runs unprompted, then try to slip past the same allowlist with shell indirection, an evaluated string, and a piped installer; each must still ask."
      - "Try to allowlist a command from the fixed irreversible set, and try to run one; confirm it can never be made automatic and that it is presented with its destructive treatment."
      - "Grant typing on one terminal and reject it on another; confirm follow-up typing on the granted one does not prompt while the other asks again, and that no configuration makes typing on a fresh terminal automatic."
      - "Find the typing grant and the remembered command shape in the grants section, revoke both, and confirm the next attempt prompts again."
    must_avoid:
      - "Do not settle who holds control during any of this — CH-terminal-lease-fencing-takeover owns the lease; do not administer the allowlist anywhere except the permissions surface that owns it."
```

## Focus areas

- **Safety Invariant 12 (execution classification)** — argv that cannot be confidently classified always
  prompts, the fixed deny set never auto-runs, and the autonomous tier never covers typing.
- **ADR-006 (permission ladder plus per-terminal typing grant)** — observe is never gated, execute is
  approved by default, autonomous execution is explicit opt-in, and typing is asked once per terminal.
- **ADR-016 (`[terminal]` configuration; policy stays in permissions)** — autonomy policy is
  administered where all agent-approval policy lives, never through terminal configuration.
- **Grant administration reuse** — the new grant kinds appear and revoke inside the existing tool-grants
  surface rather than a new administration screen of their own.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
