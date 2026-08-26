# CH-terminal-redaction-osc-boundary: Feed the terminal secrets and hostile escapes, then hunt for what it kept

```yaml
charter:
  id: CH-terminal-redaction-osc-boundary
  mission: "As Dora, push a marked secret through every hidden-input path and a hostile stream of escape sequences through the output path, then search every place the daemon retains bytes — scrollback, journal, recording, spill artifact, event payload, log, clipboard, window title — for one survivor."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-supervise-agent-terminal
  scenarios: [ET-terminal-redaction-boundaries]
  tour: Garbage Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Answer a hidden request with a uniquely greppable secret while recording is active, then grep the live scrollback, the journal row, the downloaded recording, the spill artifact, the event payloads, and the daemon log; only a length marker may appear. Repeat on the rejection path."
      - "Confirm nothing echoes during hidden entry, and that the shell's own echo behaviour afterwards is byte-for-byte what it was before — then check the same on a terminal that has no tty at all and confirm hidden input is refused there rather than degraded."
      - "Run a program that reads and writes the system clipboard through escape sequences, as controller and as watcher, and confirm neither direction reaches the real clipboard. Then confirm the filtered bytes are identical across replay, recording playback, and the byte counters."
      - "Emit a hostile window title — control characters, an over-long string, an embedded escape — and check the window, the tab, and the terminal list; then have an agent read the same screen and confirm the model-facing read additionally neutralises instruction-carrying sequences and marks the content untrusted."
      - "Write secret-shaped values into a recording and a spill artifact and confirm both are scrubbed before they land, live under the workspace's own protected directory with restrictive permissions, and refuse to resolve through a symbolic link pointing outside it."
    must_avoid:
      - "Do not settle whether the record could be written at all — CH-terminal-journal-fail-closed owns the audit-blocked path; do not settle who was allowed to answer, CH-terminal-lease-fencing-takeover owns the handoff."
```

## Focus areas

- **Safety Invariant 8 (one input choke point; redacted input is forbidden everywhere)** — every write
  path converges, and only a length marker may survive in ring, journal, recording, event payload, or
  log.
- **Safety Invariant 13 (echo guard)** — the redacted answer is delivered with echo off and the prior
  terminal state is restored immediately, and a terminal without a tty refuses hidden input instead of
  degrading.
- **Safety Invariant 14 (escape filtering sits before the buffer)** — clipboard escapes are stripped in
  both directions so replay, recording, and byte accounting see identical bytes; instruction-carrying
  sequences are additionally neutralised on the model-facing read; titles are sanitised and bounded.
- **Safety Invariant 15 (recording and spill containment)** — artifacts stay under the workspace roots
  with restrictive permissions, pass symbolic-link containment, and flow through the secret scrub before
  they are written.
- **Safety Invariant 17 (untrusted marking)** and **ADR-003 (journal always, bytes ephemeral, recording
  opt-in)** — the asymmetry that makes this safe to keep at all.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
