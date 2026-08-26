# J-operate-integrated-terminal — Work in a supervised terminal and keep agent reports distinct

An operator opens a real terminal, works in it, leaves and returns without losing the process, and
can always tell that supervised runtime activity apart from command output merely reported by an
agent.

```mermaid
flowchart TD
  A1[Entry: Terminal app] --> B[Open a terminal in the current profile]
  A2[Entry: terminal block in a session] --> C{Supervised or agent-reported?}
  B --> D[Type and observe live output]
  C -->|supervised| D
  C -->|reported by agent| R[Render inert reported output with no terminal controls]
  R --> R1[Side effect: no terminal, journal row, recording, or hook event is created]
  D --> E[Detach while the process keeps running]
  E --> F[Reopen the app and reconnect from the last acknowledged sequence]
  F --> G{Terminal still running?}
  G -->|yes| D
  G -->|exited| H[Read the final output and exit state]
  H --> Z[True end: terminal state, journal, window list, and session transcript agree on what was supervised]
  B -.->|operator closes before typing| X1[Abandon: the terminal remains available until its configured lifecycle closes it]
  E -.->|operator does not return| X2[Abandon: detached lifetime policy remains authoritative]
```

```yaml
journey:
  id: J-operate-integrated-terminal
  name: "Work in a supervised terminal and keep agent reports distinct"
  value_statement: "I can leave and return to terminal work without losing it, and I never mistake an agent's reported output for a terminal I can control."
  personas: [Marina, Ada]
  entry_points:
    - url: "Terminal app and terminal blocks in a session transcript"
      origin: in-app-nav
    - url: "Agent-reported terminal block"
      origin: agent
  actions:
    - step: 1
      verb: "Open a terminal in the current profile"
      expected_observable: "The window names the owning profile and becomes writable only after the stream is attached."
    - step: 2
      verb: "Run work, detach, and return"
      expected_observable: "Output resumes without duplicated or missing acknowledged bytes and the process keeps its identity."
    - step: 3
      verb: "Inspect an agent-reported command block"
      expected_observable: "It is labeled reported by agent, has no control affordances, and creates no supervised terminal state."
    - step: 4
      verb: "Let the terminal exit and re-read its state"
      expected_observable: "The final output, exit reason, journal, and retained window state agree after a fresh load."
  goal:
    observable: "Supervised terminals remain resumable and agent reports remain inert across the session transcript, Terminal app, journal, and window catalog."
    side_effects: [terminal-created, stream-attached, journal-written, terminal-exit-retained]
  true_end_state: "A fresh read shows one coherent supervised lifecycle, while reported-only output appears solely in the conversation and leaves terminal storage empty."
  exit:
    natural: "The operator closes the finished terminal or continues work in the resumed one."
  abandonment:
    - at_step: 2
      how: "Close the window while the command is still running."
      resume: "Reopen the Terminal app and attach to the same terminal from the current stream cursor."
  crosses: [terminal-runtime, session-transcript, journal, recordings, profile-scope, WebSocket-stream]
```
