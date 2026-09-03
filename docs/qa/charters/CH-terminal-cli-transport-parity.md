# CH-terminal-cli-transport-parity: Drive the whole terminal from structured surfaces and find where two of them disagree

```yaml
charter:
  id: CH-terminal-cli-transport-parity
  mission: "As Ada, complete a full terminal lifecycle using only structured surfaces — every command-line verb, every route on both transports, the catalog stream, and the agent toolset — and hunt for one field, one error code, or one event that two of them describe differently."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-terminal-by-cli
  scenarios: [ET-terminal-cli-public-contract, ET-terminal-hook-events]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Exercise every terminal verb: open attached and detached, execute, list, get, attach to watch and to control, kill, signal, list and answer input requests, read the journal, start and stop recording, and quote a range. Record which verbs you did not reach."
      - "Compare every non-interactive result against the matching route on both transports — list, create, execute, input requests, journal, recording and artifact downloads, detail, delete, attach ticket, read, signal, wait, answer, reject, and recording control — field by field, not just status code by status code."
      - "Open the catalog stream and a per-terminal stream on both transports; check initial state, a live update, replay from a cursor, and how each closes."
      - "Walk the documented failures deliberately — bad flags, bad selectors, wrong terminal state, unsupported capability — and compare exact codes across surfaces; a code that differs between two surfaces is the finding."
      - "Drive every lifecycle transition and confirm each emits exactly one event in per-terminal order with the owning terminal, workspace, profile, actor, run, and command fields — and that output an agent merely reported emits none."
      - "Attach interactively in watch and in control mode and confirm the banner, the detach chord, the single-key passthrough, and the refusal on an exited terminal, without expecting structured output from an interactive stream."
    must_avoid:
      - "Do not settle profile selector grammar — CH-terminal-profile-fence owns it; do not settle stream degradation under load, CH-terminal-stream-flow-control owns that."
```

## Focus areas

- **Public contract completeness** — twelve command-line verbs, seventeen routes plus the catalog
  stream, and eleven agent tools, each reachable and each agreeing on one projection of a terminal.
- **ADR-008 (one stream per terminal plus a catalog stream)** — the catalog keeps lists and badges live
  without requiring anyone to be attached.
- **Agent manageability** — the structured surfaces are first-class operation paths, not a mirror of the
  browser; anything only reachable through the browser is a finding.
- **Event coverage (US-032)** — every lifecycle transition emits exactly once, in per-terminal order,
  carrying identities and outcomes but never terminal bytes or redacted input, and a slow handler never
  makes the terminal wait.
- **Deterministic error codes** — the same condition produces the same code on the command line, both
  transports, and the agent tools.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
