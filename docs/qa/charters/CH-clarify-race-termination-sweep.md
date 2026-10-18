# CH-clarify-race-termination-sweep: Concurrent answers, cancels, and stops settle a clarification exactly once

```yaml
charter:
  id: CH-clarify-race-termination-sweep
  mission: "As Ada, race answers against cancels and stops from different structured surfaces and prove exactly one terminal outcome wins with deterministic loser errors and intact CLI/HTTP/UDS parity."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-answer-agent-requests
  scenarios: [RT-session-clarification-roundtrip]
  tour: Multi-Tab Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Race an answer against cancel and against session stop from different surfaces (CLI vs HTTP/UDS); exactly one terminal outcome wins and each loser reports its deterministic not-found or conflict error."
      - "Ask twice on one session; the second ask conflicts naming the live request while the first wait and its pings continue undisturbed."
      - "Answer an unknown or finished request and probe cross-workspace list/answer; each denies deterministically and no ping ever crosses sessions."
      - "Diff CLI -o json against HTTP/UDS pending and answer payloads for deadline null and receipt identity."
    must_avoid:
      - "The web timeline — parity under race is a structured-surface contract here."
      - "Settling policy-matrix verdicts — invalid values and restart behavior belong to CH-clarify-policy-matrix."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's report. -->
