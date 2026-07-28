# J-watch-agent-work-channel — Watch the agent work without channel noise

A teammate triggers a tool-heavy turn and follows one bounded, readable status lifecycle to a final
answer in the same conversation. The journey covers default-on editable providers, opted-in
lower-tier providers, append-only WhatsApp, mode-off silence, and issue-provider no-op progress.

```mermaid
flowchart TD
    E[Entry: mention or message the agent] --> M{Resolved progress mode}
    M -->|off| O[No progress provider calls]
    O --> OF[Final answer appears in the triggering conversation]
    M -->|new / all / verbose| P{Provider capability}
    P -->|Slack / Telegram / Discord| P1[Post one threaded progress bubble]
    P -->|Teams / Google Chat opted in| P2[Post then edit one status message]
    P -->|WhatsApp opted in| P3[Post sparse append-only status lines]
    P -->|GitHub / Linear| P4[Acknowledge with no issue-side progress]
    P1 --> T[Rapid tool events enter throttle and dedup]
    P2 --> T
    P3 --> T
    T --> E1{Tool result}
    E1 -->|success| E2[Completion line and supported reaction]
    E1 -->|failure| E3[Short redacted failure line]
    E2 --> C{More tools?}
    E3 --> C
    C -->|yes| T
    C -->|no| F[Typing stops; final answer posts separately]
    P4 --> F
    F --> R[Fresh session read contains answer but no progress chrome]
    R --> Z[True end: teammate understands the outcome without spam or thread drift]
    T -.->|abandon because edits flood or leave the thread| X[Teammate mutes or closes the conversation]
    X -.->|resume| XR[Return to one bounded thread with terminal state and final answer]
    XR --> Z
```

```yaml
journey:
id: J-watch-agent-work-channel
  name: "Watch the agent work without channel noise"
  value_statement: "I can tell that the agent is working and what finished without being flooded or leaving the conversation that started the work."
  personas: [Maya, Omar]
  entry_points:
    - url: "Slack, Telegram, Discord, Teams, Google Chat, or WhatsApp bridge conversation"
      origin: external-share
    - url: "GitHub issue/review comment or Linear comment/Agent Session"
      origin: external-share
  actions:
    - step: 1
      verb: "Trigger a turn that runs several repeated and distinct tools"
      expected_observable: "The first supported progress surface appears in the triggering conversation, or mode off remains silent"
    - step: 2
      verb: "Watch rapid starts, completions, and one failure"
      expected_observable: "Updates are throttled, identical consecutive lines collapse, and the terminal state is never dropped"
    - step: 3
      verb: "Let the final answer settle"
      expected_observable: "Typing and mutable progress stop; the final answer remains a distinct message in the correct thread"
    - step: 4
      verb: "Open the corresponding session transcript"
      expected_observable: "Only agent/session events are present; channel progress chrome is absent"
  goal:
    observable: "The final answer and terminal tool outcome are understandable at the intended target with no secret, duplicate terminal, or cross-thread update."
    side_effects: [progress-posted-or-nooped, progress-edited-or-appended, typing-cleared, final-answer-delivered]
  true_end_state: "After the turn and a fresh transcript read, the channel has bounded presentation state, the final answer is in the right conversation, and transcript history is pure."
  exit:
    natural: "The teammate reads the answer and continues the same conversation."
  abandonment:
    - at_step: 2
      how: "The teammate mutes or leaves because status updates appear too frequently or in the wrong thread."
      resume: "Returning after the turn shows a bounded terminal status and one final answer rather than an unbounded edit storm."
  crosses: [acp-events, canonical-redaction, ordered-delivery, provider-rate-limits, provider-formatters, channel-threading, session-transcript]
```

Automated backbone: `_tests.md` E2E runtime 6.1–6.3 plus provider fake-API suites. Task 10 owns
the observable spam, routing, and transcript-purity walk.
