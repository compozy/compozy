# CH-loop-legibility-run-default-read: Pass the briefing test with every disclosure closed

```yaml
charter:
  id: CH-loop-legibility-run-default-read
  mission: "As Lea, open runs I did not start — running, blocked, failed, terminal, no-op — and prove I can say what each one is doing, what it needs, how far it got, what it cost and what it produced, in under thirty seconds, without expanding anything and without reading a single machine id."
  mode: charter-with-tour
  persona:
    name: Lea
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-supervise-loop-steady-state
  scenarios: [LP-web-run-default-read-briefing, LP-web-runs-roster-rerank, LP-run-detail-story-redesign, LP-web-strategy-progress]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Open the roster first: needs-you runs lead, outcomes are in plain words, the run id is secondary, and Progress comes from the served values — watch the network and confirm one list read with no per-row follow-up."
      - "Time the read. Open a run at an approval gate with everything collapsed and answer the five questions aloud — what is running, what needs me, how far along, what has it spent, what did it produce. Anything that forces a disclosure to answer is a finding, not a preference."
      - "Confirm the needs-you card is the page's only primary — the briefing strip carries no Approve/Reject, only the quiet pointer to the card — that failure and needs-you never collapse, and that the main column holds no machine ids (no loop. prefix, no looprun- id); the run id may appear only as the About rail's labelled Run row."
      - "Let the run finish and reload: outcome and produced artifacts must lead. Then walk a pruned artifact (keeps its name, says content is no longer stored), a run that produced nothing (says so plainly), a failed run (failure visible with everything collapsed), and a fan-out run where a strategy cancellation reads differently from a failure, a never-materialized lane differently from a pending one, the partial badge carries coverage numbers, and a wide fan-out reports aggregates instead of per-lane rows. Throughout, read the story for enum leaks: beats are sentences, heartbeat-class events coalesce as one xN entry, and no runtime reason code prints under its own label."
    must_avoid:
      - "Expanding Inspect — the operator register belongs to CH-loop-legibility-operator-register and opening it invalidates this session's central claim."
      - "Grading copy against the spec's wording instead of against whether a person who did not start the run understands it."
```

## Selection rationale

Targeted tier — the supervisor half of the two-register briefing test. Owns Safety Invariant 12 (the
verdict is computed server-side from the same reads the page renders; web never re-derives one) and
ADR-002's single page with two registers and no mode toggle. The Feature Tour is deliberate: the
headline promise here *is* the default read, so the session's job is to walk the advertised path and
check the promise, not to break it. This charter and its sibling operator charter together carry
`BUG-20260719-autonomous-progress-unobservable` — the P1 whose whole symptom is an operator unable
to tell autonomous progress from a stall. The three adjacent scenarios ride along because task_05's
redesign reset them: the story, the progress panel and the roster are the blast radius of the same
diff.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
