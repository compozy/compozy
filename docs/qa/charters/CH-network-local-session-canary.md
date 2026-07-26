# CH-network-local-session-canary: Canary the ordinary session lifecycle beside the Network cut

```yaml
charter:
  id: CH-network-local-session-canary
  mission: "As Ada, run an ordinary session lifecycle over CLI, HTTP, and UDS while Network is available, proving transcript and lifecycle parity still hold and no Network dependency or artifact appears."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-15
  scenarios: [RT-023, RT-042]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Bootstrap a fresh isolated lab with unique AGH_HOME/ports/provider home/tmux socket, register PIDs, and execute eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every pass/fail/blocked/abort exit; cite clean teardown.json."
      - "With Network available but participation omitted, create, prompt, background, follow, stop, restart the daemon, and compare list/detail/status/transcript output over CLI, HTTP, and UDS."
      - "Independently read Network status, channels, wakes, and usage before and after. The session must remain Local and the ordinary transcript/lifecycle path must not depend on a conversation."
    must_avoid:
      - "Turning the canary Live, re-running CH-018's exhaustive transcript matrix, using the Web UI, or exploring mailbox/spend-cap scope."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief belongs in the dated report. -->
