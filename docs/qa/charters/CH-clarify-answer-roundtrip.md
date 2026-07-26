# CH-clarify-answer-roundtrip: A live agent question blocks, gets my answer, and tells the truth after

```yaml
charter:
  id: CH-clarify-answer-roundtrip
  mission: "As Théo, answer live agh__clarify questions from every public surface, force the timeout sentinel, and verify pending state, receipts, isolation, and the keyboard path all tell the truth."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-answer-agent-requests
  scenarios: [RT-session-clarification-roundtrip]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Answer a 4-choice question from the Web card, then a free-text extension-tool question via CLI and HTTP; the tool result must carry the exact answer and unblock the turn."
      - "Let one question time out → the unanswered sentinel (Choice=nil, Text empty, Fallback=true) treated as a non-answer, never a synthesized selection."
      - ">4 choices → validation error; a second simultaneous question → deterministic rejection; another workspace can neither list nor answer."
      - "Reload after resolved/timeout/cancel transitions — receipts stay truthful and distinct from permission prompts; drive the card keyboard-only and through a refresh while pending (experiential sweep)."
    must_avoid:
      - "Mutating approval state through clarify — it is a question channel, not a prompt class."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
