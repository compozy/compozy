# CH-session-command-catalog-parity: Compare the effective command catalog across public surfaces

```yaml
charter:
  id: CH-session-command-catalog-parity
  mission: "As Bruno, inspect one active session from Web, CLI, and HTTP and trust that every surface lists the same usable commands without exposing another workspace."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-use-session-slash-commands
  scenarios: [ET-session-command-catalog-parity]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open the Web command menu, record visible command tokens, and list the same session with compozy session commands <session-id> -o json."
      - "Read the documented HTTP command route and compare revision, ids, tokens, lanes, source kinds, source ids, scopes, and descriptions."
      - "Repeat the HTTP read with a different workspace id and confirm it returns not found without catalog data."
      - "Refresh the Web surface and repeat one CLI read to prove the result is not a one-frame optimistic projection."
    must_avoid:
      - "Do not read the database or internal Go state; only Web, CLI, and the documented HTTP route count."
      - "Do not claim live native-tool provider coverage from this non-provider targeted lab."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
