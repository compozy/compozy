# CH-prompt-bound-runtime-transition: Change runtime at prompt boundaries without rewriting history

```yaml
charter:
  id: CH-prompt-bound-runtime-transition
  mission: "As Théo, use the session composer and public prompt surfaces to prove every prompt keeps its chosen runtime snapshot through live reconfiguration, process replacement, queueing, and a later runtime change."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-13
  scenarios: [RT-018, RT-019, RT-059, RT-061, RT-064, RT-065, RT-066, RT-067, RT-070, RT-072, RT-session-prompt-runtime-transitions]
  tour: Multi-Tab Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Create a logical session, select an advertised provider, model, reasoning level, and speed in Next prompt, then send it. Independently compare the prompt's transcript/runtime evidence with the ACP ordering evidence."
      - "Send a later prompt with a different runtime and prove the prior prompt snapshot and the agent default remain unchanged; include Claude max and one model switch that clears an unsupported reasoning level."
      - "Keep the session open in two tabs while a turn runs. Queue two drafts with different runtime snapshots, edit or remove one, then prove dispatched order and snapshots survive the settle in both tabs."
      - "Exercise a supported live reconfiguration and a replacement-required transition. Force one transition failure and prove its prompt was not dispatched while the prior runtime still accepts a valid prompt."
    must_avoid:
      - "Do not use browser storage or DevTools as proof of favorites/recents; reopen the selector after refresh to verify persisted UI behavior."
      - "Do not settle unavailable-model or unsupported-reasoning error codes here; CH-prompt-runtime-fail-loud owns the typed negative contract."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
