# CH-terminal-shared-input-race: Let every participant work in one terminal at once

```yaml
charter:
  id: CH-terminal-shared-input-race
  mission: "As Marina supervising live agent work, use the same terminal from two browser contexts, two command-line attachments, and an authorized agent, trying to find any remaining handoff, rejected peer, fragmented submission, false attribution, or stale controller state."
  mode: charter-with-tour
  persona:
    name: Marina
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-supervise-agent-terminal
  scenarios: [ET-terminal-shared-control, ET-terminal-stream-resilience, ET-terminal-browser-lifecycle, ET-terminal-session-block-handoff]
  tour: Multi-Tab Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Attach two independent browser contexts and two interactive CLI clients to one running terminal; type from each without clicking or requesting any control action."
      - "Submit distinct newline-terminated markers close together from every participant; confirm each arrives exactly once and whole, then compare the actor attribution through a public journal read."
      - "Keep an agent writing while an operator resizes, answers one hidden request, sends a signal, and closes from another client; every authorized mutation must work without claim, yield, takeover, or a typing grant."
      - "Open one explicit read-only presentation attachment and confirm it can follow output and presence without mutating or reducing anyone else's access."
      - "Disconnect and reconnect one browser context during output, reload the Terminal window, and open the supervised session block; output, presence, exit state, and the existing terminal identity must converge."
      - "Search every rendered terminal surface for controller, takeover, release, claim, yield, and typing-grant affordances; none may remain."
    must_avoid:
      - "Do not use a private database or implementation state to settle the result; drive and verify through Web, CLI, documented API, native tools, and journal reads from the isolated lab."
```

## Focus areas

- Shared mutation is the default for every authorized interactive participant in one workspace/profile.
- Each submitted input remains atomic and journal attribution follows the sender whose submission
  completed the command line.
- Explicit read-only presentation views stay passive without becoming an ownership mechanism.
- Sensitive input remains redacted and is settled at most once even when several participants can act.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
