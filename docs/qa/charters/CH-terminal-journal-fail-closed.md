# CH-terminal-journal-fail-closed: Break the command record and see whether work still runs

```yaml
charter:
  id: CH-terminal-journal-fail-closed
  mission: "As Rafa auditing what ran, make the command record store fail underneath a live terminal and try to get one command executed or one keystroke accepted that never made it into the record — then restore the store and confirm the terminal recovers by itself with no gap and no duplicate row."
  mode: charter-with-tour
  persona:
    name: Rafa
    device: desktop
    network: flaky
    locale: en-US
  journey: J-audit-terminal-work
  scenarios: [ET-terminal-journal-fail-closed, ET-terminal-journal-recording]
  tour: Network Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "With a terminal running, make the record store fail; try to run a new command and to send new input, and confirm both are refused with a reason that names the record as the blocker rather than a generic failure."
      - "During that window confirm existing output stays readable, watchers stay attached, and nothing already running is killed — the refusal must be narrow, not a shutdown."
      - "Restore the store; confirm pending records land, the terminal accepts work again without a restart, and the resulting history has every command that actually ran, with no gap and no row for a blocked attempt."
      - "Turn command-boundary marking off in configuration and confirm the record keeps working with its boundaries honestly labelled approximate; run one exactly-detected and one approximated command and confirm both reads distinguish them."
      - "Filter by actor, time, terminal, and failure; confirm a filtered miss is visibly different from a project with no history, that every row names actor, approval, and owning profile, and that a recording replays from its owning profile with its retention stated."
      - "Select a bounded range of terminal output, send it to the active conversation, and confirm the source terminal, the line range, and the untrusted marking all survive being sent."
    must_avoid:
      - "Do not settle what a redacted answer leaves behind — CH-terminal-redaction-osc-boundary owns that; use the isolated lab from the bootstrap manifest, never the default COMPOZY_HOME or ports."
```

## Focus areas

- **ADR-003 (journal always, bytes ephemeral, recording opt-in)** — the record is the durable half of the
  audit posture, so it fails closed: no command runs unrecorded.
- **Fail-closed audit state** — an audit-blocked terminal refuses new input and new commands with its own
  reason while output, reads, and watching continue, and clears itself when the pending rows land.
- **ADR-014 and ADR-018 (journal storage and profile integrity enforced in code)** — the record lives in
  the workspace store while the owner lives elsewhere, so the owner link is enforced rather than assumed.
- **Honest completion detection** — with boundary marking off, rows degrade to approximate and say so;
  nothing else breaks and nothing is presented as exact that was guessed.
- **Session bridge integrity** — a quoted excerpt keeps its terminal identity, its line range, and its
  untrusted status when it enters a conversation.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
