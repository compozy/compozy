# CH-terminal-operator-shell: Live in the terminal window for an hour and never lose work or mistake a report for a terminal

```yaml
charter:
  id: CH-terminal-operator-shell
  mission: "As Bruno, do a normal hour of project work inside the Terminal app — open, tab, detach, return, let one exit — and confirm the window, the dock, the session transcript, and the terminal list never disagree about what is running and what was only the agent's internal command output."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-integrated-terminal
  scenarios: [ET-terminal-browser-lifecycle, ET-terminal-session-block-handoff, ET-agent-terminal-window-materialization]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Start from a project with no terminals: use the empty-state action, open a second terminal, switch tabs, reload the browser, and confirm both survive with their scrollback."
      - "Close the Terminal window while a command is still running, reopen the app, and reattach; then let a terminal exit and read its final output and exit cause, including a cause the daemon reports as unknown."
      - "Ask an agent for one visible command and follow its transcript block: watch it stream, use the open affordance, and confirm it focuses the existing terminal instead of opening a second one."
      - "Run routine agent-internal commands and one supervised terminal command in the same transcript and confirm the internal ones render as plain command output while only the supervised run owns a terminal, a window, and journal rows."
    must_avoid:
      - "Do not settle stream degradation, viewer limits, or reconnect gaps — CH-terminal-stream-flow-control owns those; do not use the default COMPOZY_HOME or ports, use the isolated lab from the bootstrap manifest."
```

## Focus areas

- **ADR-001 (deliberate surface, not default route)** — the agent reaches for a terminal on direction or
  on a named trigger; routine internal work stays session activity and must not create a terminal, a
  journal row, or a window.
- **ADR-005 (project-owned terminals, optional run binding)** — a terminal belongs to the project and
  outlives the window that opened it; a run binding never becomes ownership.
- **Safety Invariant 10 (process registration and drain)** — every terminal is reachable by the single
  drain path, and reclaiming is driven by activity, not by how long ago it was spawned.
- **Safety Invariant 17 (terminal output is untrusted data)** — everything the transcript feeds back to
  the model carries the untrusted marking.
- **Truthful-UI directive** — an exit cause the daemon does not know renders as unknown, and a block
  agent-internal command output never grows terminal chrome or controls.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
