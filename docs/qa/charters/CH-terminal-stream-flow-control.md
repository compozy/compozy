# CH-terminal-stream-flow-control: Crowd one terminal with viewers and break their connections

```yaml
charter:
  id: CH-terminal-stream-flow-control
  mission: "As Marina, attach many viewers to one noisy terminal across tabs and clients, then stall, throttle, and cut them one at a time, trying to make the program freeze, a viewer receive output that never happened, or a gap pass unannounced."
  mode: charter-with-tour
  persona:
    name: Marina
    device: desktop
    network: flaky
    locale: en-US
  journey: J-operate-integrated-terminal
  scenarios: [ET-terminal-stream-resilience]
  tour: Multi-Tab Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Attach one controlling viewer and several watchers across separate tabs and a command-line attach, then flood output and confirm only the controlling viewer can ever slow the program down."
      - "Stall one watcher — background its tab, suspend it, or throttle it hard — and confirm the others and the program keep going, and that the stalled one is either given a stated gap or disconnected with a stated reason."
      - "Keep a viewer over its budget long enough to be demoted, then confirm the demotion is announced as a gap rather than silently swallowing output, and that memory does not keep growing behind it."
      - "Cut and restore the network mid-command for one viewer; confirm it resumes with no duplicated and no missing acknowledged output, and repeat with a replay cursor older than the retained window to confirm it restarts from a full snapshot."
      - "Resize from the smallest controlling viewer and confirm watchers cannot shrink it; then reuse an attach ticket, use an expired one, and use one minted for another terminal or mode, and confirm each is refused before the connection opens."
    must_avoid:
      - "Do not settle viewer-cap admission or its recovery message — CH-terminal-capacity-and-config owns that; do not settle lease ownership questions, CH-terminal-lease-fencing-takeover owns those."
```

## Focus areas

- **Safety Invariant 6 (watchers never influence control)** — attaching, detaching, or being evicted as
  an observer can never change who holds control.
- **Safety Invariant 7 (only acknowledging subscribers throttle the program)** — a dropping viewer
  overflows into a gap or an eviction and never applies back pressure to the running program.
- **Safety Invariant 11 (single-use attach tickets)** — a reused, expired, foreign, or wrong-mode ticket
  fails before the connection is established, and tickets die with their terminal.
- **ADR-008 (one stream per terminal plus a catalog stream)** and **ADR-009 (acknowledging and dropping
  flow classes)** — the multi-viewer rule that one stuck viewer can neither freeze the others nor grow
  memory without bound.
- **ADR-010 (ring replay)** — replay resumes at a safe boundary, and a cursor past the retained window
  restarts honestly instead of serving a partial history as if it were complete.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
