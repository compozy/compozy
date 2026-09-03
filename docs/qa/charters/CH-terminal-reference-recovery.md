# CH-terminal-reference-recovery: Reach a visible terminal from an ordinary prompt

```yaml
charter:
  id: CH-terminal-reference-recovery
  mission: "As Bruno, ask for a short terminal demonstration in ordinary language and watch the agent reach a visible supervised terminal even when a skill reference read cannot be completed."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: pt-BR
  journey: J-operate-integrated-terminal
  scenarios: [ET-agent-terminal-window-materialization]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Use the original Portuguese request with no QA framing: ask the agent to use Compozy's embedded terminal and perform a few harmless operations."
      - "Observe the skill/resource calls, the visible terminal creation, focus behavior, live output, and a fresh terminal list read for the same terminal id."
      - "Close the window while the process remains alive, then confirm it does not reopen and routine agent-internal commands do not create terminal windows."
    must_avoid:
      - "Do not tell the agent which native tool to call or how the recovery rule works; that would invalidate the prompt-fidelity result."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
